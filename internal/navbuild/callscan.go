package navbuild

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// extract_calls_from_zone of build_callgraph_index.py. Its regular
// expressions are matched by hand with Python's Unicode \w, \s and \b
// (pycompat.IsWordRune, pycompat.IsSpace); each is unambiguous (a greedy
// word run can only shrink onto another word character), so trying every
// start position in order and resuming after a match is re.finditer.

// skipReceivers is SKIP_RECEIVERS: Java library classes whose calls are noise.
var skipReceivers = setOf(`System String Integer Long Double Float Boolean
	Byte Short Character Number Math StrictMath
	Arrays Collections Objects Class Thread Runtime
	StringBuilder StringBuffer Object Enum Void
	Comparable Iterable Iterator Spliterator
	Map Set List Queue Deque Collection
	HashMap HashSet ArrayList LinkedList TreeMap TreeSet
	LinkedHashMap LinkedHashSet ConcurrentHashMap Vector
	CopyOnWriteArrayList CopyOnWriteArraySet
	Optional Stream Collectors IntStream LongStream
	Pattern Matcher
	Calendar Date SimpleDateFormat TimeZone Locale
	File Path Paths Files
	Logger Level LogManager LogRecord
	Assert Preconditions
	InputStream OutputStream Reader Writer
	BufferedReader BufferedWriter InputStreamReader OutputStreamWriter
	FileInputStream FileOutputStream ByteArrayInputStream ByteArrayOutputStream
	DataInputStream DataOutputStream ObjectInputStream ObjectOutputStream
	PrintStream PrintWriter
	Socket ServerSocket URL URI HttpURLConnection
	Properties ResourceBundle
	BigDecimal BigInteger
	UUID Random SecureRandom
	Throwable Exception RuntimeException Error
	IOException IllegalArgumentException IllegalStateException
	NullPointerException UnsupportedOperationException
	ClassNotFoundException NoSuchMethodException
	Field Method Constructor Modifier
	Proxy InvocationHandler
	Annotation Override Deprecated SuppressWarnings`)

// skipNew is SKIP_NEW: common allocations not recorded as constructor calls.
var skipNew = setOf(`String Integer Long Double Float Boolean Byte Short
	Character BigDecimal BigInteger
	StringBuilder StringBuffer
	ArrayList LinkedList HashMap HashSet TreeMap TreeSet
	LinkedHashMap LinkedHashSet ConcurrentHashMap Vector
	CopyOnWriteArrayList CopyOnWriteArraySet
	Object Exception RuntimeException
	IOException IllegalArgumentException IllegalStateException
	NullPointerException UnsupportedOperationException
	File URL URI
	Date Calendar SimpleDateFormat Locale TimeZone
	Timer TimerTask
	Thread ThreadLocal
	Properties Random SecureRandom UUID
	ByteArrayInputStream ByteArrayOutputStream
	BufferedReader BufferedWriter
	InputStreamReader OutputStreamWriter
	PrintWriter PrintStream
	FileInputStream FileOutputStream
	DataInputStream DataOutputStream
	ObjectInputStream ObjectOutputStream
	StringReader StringWriter
	Pattern Matcher`)

// notCalls is NOT_CALLS: keywords that look like calls.
var notCalls = setOf(`if else while for do switch try catch finally
	return throw new super this case import package
	class interface enum assert break continue goto
	instanceof var record sealed permits yield null
	true false void synchronized`)

// wordRun is the end of the \w+ run at i (i when there is none).
func wordRun(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !pycompat.IsWordRune(r) {
			break
		}
		i += n
	}
	return i
}

// spaces is the end of the \s* run at i.
func spaces(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !pycompat.IsSpace(r) {
			break
		}
		i += n
	}
	return i
}

// wordStart is \b before a word character at i.
func wordStart(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !pycompat.IsWordRune(r)
}

// finditer calls match at every start position of s in order; a match
// returns its end, where the scan resumes, or -1.
func finditer(s string, match func(i int) int) {
	for i := 0; i < len(s); {
		if end := match(i); end >= 0 {
			i = max(end, i+1)
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
	}
}

// keywordCall matches \bkw\.(\w+)\s*\( at i: the name and the match end.
func keywordCall(s string, i int, kw string) (string, int) {
	if !strings.HasPrefix(s[i:], kw+".") || !wordStart(s, i) {
		return "", -1
	}
	j := i + len(kw) + 1
	k := wordRun(s, j)
	if k == j {
		return "", -1
	}
	if e := spaces(s, k); e < len(s) && s[e] == '(' {
		return s[j:k], e + 1
	}
	return "", -1
}

// keywordParen matches \bkw\s*\( at i: the match end.
func keywordParen(s string, i int, kw string) int {
	if !strings.HasPrefix(s[i:], kw) || !wordStart(s, i) {
		return -1
	}
	if e := spaces(s, i+len(kw)); e < len(s) && s[e] == '(' {
		return e + 1
	}
	return -1
}

// newCall matches \bnew\s+(\w+)\s*[\(<] at i: the class and the match end.
func newCall(s string, i int) (string, int) {
	if !strings.HasPrefix(s[i:], "new") || !wordStart(s, i) {
		return "", -1
	}
	j := spaces(s, i+3)
	if j == i+3 {
		return "", -1
	}
	k := wordRun(s, j)
	if k == j {
		return "", -1
	}
	if e := spaces(s, k); e < len(s) && (s[e] == '(' || s[e] == '<') {
		return s[j:k], e + 1
	}
	return "", -1
}

// dotCall matches (\w+)\.(\w+)\s*\( at i: receiver, method and match end.
func dotCall(s string, i int) (string, string, int) {
	j := wordRun(s, i)
	if j == i || j >= len(s) || s[j] != '.' {
		return "", "", -1
	}
	k := wordRun(s, j+1)
	if k == j+1 {
		return "", "", -1
	}
	if e := spaces(s, k); e < len(s) && s[e] == '(' {
		return s[i:j], s[j+1 : k], e + 1
	}
	return "", "", -1
}

// blankLiterals is the two re.sub calls that empty "..." and '...' literals
// (a backslash escapes any character; an unterminated quote stays).
func blankLiterals(s string) string {
	for _, q := range []byte{'"', '\''} {
		if strings.IndexByte(s, q) < 0 {
			continue
		}
		var b strings.Builder
		for i := 0; i < len(s); {
			if s[i] == q {
				if end := literalEnd(s, i, q); end >= 0 {
					b.WriteByte(q)
					b.WriteByte(q)
					i = end
					continue
				}
			}
			b.WriteByte(s[i])
			i++
		}
		s = b.String()
	}
	return s
}

// literalEnd matches q[^q\\]*(?:\\.[^q\\]*)*q at i: its end, or -1.
func literalEnd(s string, i int, q byte) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case q:
			return j + 1
		case '\\':
			if j+1 >= len(s) || s[j+1] == '\n' {
				return -1
			}
			_, n := utf8.DecodeRuneInString(s[j+1:])
			j += n
		}
	}
	return -1
}

// isUpperFirst is name[0].isupper().
func isUpperFirst(name string) bool {
	r, n := utf8.DecodeRuneInString(name)
	return n > 0 && pycompat.IsUpper(string(r))
}

// zoneCalls is extract_calls_from_zone: the calls of lines made from class
// current, whose direct parent (for super) is parent ("" for none).
func zoneCalls(lines []string, current, parent string) map[string]bool {
	calls := map[string]bool{}
	target := parent
	if target == "" {
		target = current
	}
	inBlock := false
	for _, line := range lines {
		s := pycompat.Strip(line)
		if s == "" {
			continue
		}
		if inBlock {
			end := strings.Index(s, "*/")
			if end < 0 {
				continue
			}
			s, inBlock = s[end+2:], false
		}
		s, inBlock = dropBlockComments(s, inBlock)
		if c := strings.Index(s, "//"); c >= 0 {
			s = s[:c]
		}
		if s == "" {
			continue
		}
		s = blankLiterals(s)
		finditer(s, func(i int) int {
			name, end := keywordCall(s, i, "this")
			if end >= 0 && !notCalls[name] {
				calls[current+"."+name] = true
			}
			return end
		})
		finditer(s, func(i int) int {
			name, end := keywordCall(s, i, "super")
			if end >= 0 && !notCalls[name] {
				calls[target+"."+name] = true
			}
			return end
		})
		for _, kw := range []string{"super", "this"} {
			owner := target
			if kw == "this" {
				owner = current
			}
			finditer(s, func(i int) int {
				end := keywordParen(s, i, kw)
				if end >= 0 && !strings.HasSuffix(strings.TrimRightFunc(s[:i], pycompat.IsSpace), ".") {
					calls[owner+".<init>"] = true
				}
				return end
			})
		}
		finditer(s, func(i int) int {
			name, end := newCall(s, i)
			if end >= 0 && isUpperFirst(name) && !skipNew[name] && !notCalls[name] {
				calls[name+".<init>"] = true
			}
			return end
		})
		finditer(s, func(i int) int {
			recv, name, end := dotCall(s, i)
			if end >= 0 && recv != "this" && recv != "super" && !notCalls[name] &&
				!skipReceivers[recv] && isUpperFirst(recv) {
				calls[recv+"."+name] = true
			}
			return end
		})
	}
	return calls
}
