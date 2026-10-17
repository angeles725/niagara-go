package bm25

import "math"

// The natural logarithms of the Python tools, bit for bit. CPython's math.log
// and math.log1p call glibc; on x86-64 with FMA (every host the tools run on)
// glibc dispatches to __log_fma and __log1p_fma. Go's math.Log differs from
// __log_fma by one ulp on ~1.4 % of BM25 idf inputs and math.Log1p from
// __log1p_fma on 66 of the integers 0..2,000,000 (175 and 184 among them), which
// moves scores and can swap near-ties. These ports reproduce glibc 2.39's
// results; testdata/fixtures.json pins them against CPython.
//
// The fused operations follow glibc's machine code (objdump of libm.so.6), not
// its C text: GCC contracts a*b+c freely when it builds the FMA variants. Every
// other product stands alone in its own statement or operand, and Go fuses only
// an explicit math.FMA on amd64 below GOAMD64=v3; a product that feeds an
// addition directly is wrapped in float64(...), which forbids fusion anywhere.

// Log is glibc's __log_fma (Arm optimized-routines) for every float64.
func Log(x float64) float64 {
	const (
		lo  = 0x3fee000000000000 // asuint64(1.0 - 0x1p-4)
		hi  = 0x3ff1090000000000 // asuint64(1.0 + 0x1.09p-4)
		off = 0x3fe6000000000000
	)
	ix := math.Float64bits(x)
	if ix-lo < hi-lo {
		return logNearOne(x, ix)
	}
	top := uint32(ix >> 48)
	if top-0x0010 >= 0x7ff0-0x0010 {
		switch {
		case ix*2 == 0:
			return math.Inf(-1)
		case ix == math.Float64bits(math.Inf(1)):
			return x
		case top&0x8000 != 0 || top&0x7ff0 == 0x7ff0:
			return math.NaN()
		}
		// Subnormal: normalize x.
		ix = math.Float64bits(x * 0x1p52)
		ix -= 52 << 52
	}
	tmp := ix - off
	i := (tmp >> (52 - 7)) % 128
	k := int64(tmp) >> 52
	iz := ix - (tmp & (0xfff << 52))
	invc, logc := logTab[i][0], logTab[i][1]
	z := math.Float64frombits(iz)
	kd := float64(k)
	a := logPoly
	w := math.FMA(kd, logLn2hi, logc)
	r := math.FMA(z, invc, -1.0)
	h := r + w
	r2 := r * r
	l := math.FMA(kd, logLn2lo, w-h+r)
	inner := math.FMA(math.FMA(r, a[4], a[3]), r2, math.FMA(r, a[2], a[1]))
	return math.FMA(r*r2, inner, math.FMA(r2, a[0], l)) + h
}

// logNearOne is the |x - 1| < 0x1p-4 branch, with __log_fma's fusions.
func logNearOne(x float64, ix uint64) float64 {
	if ix == math.Float64bits(1.0) {
		return 0
	}
	b := logPoly1
	r := x - 1.0
	r2 := r * r
	r3 := r * r2
	t3 := math.FMA(r3, b[10], math.FMA(r2, b[9], math.FMA(r, b[8], b[7])))
	t2 := math.FMA(t3, r3, math.FMA(r2, b[6], math.FMA(r, b[5], b[4])))
	poly := math.FMA(t2, r3, math.FMA(r2, b[3], math.FMA(r, b[2], b[1])))
	rhi := math.FMA(-0x1p27, r, math.FMA(r, 0x1p27, r)) // r + w - w, w = r * 0x1p27
	rlo := r - rhi
	sq := rhi * rhi
	h := math.FMA(sq, b[0], r)
	l := math.FMA(sq, b[0], r-h)
	l = math.FMA(b[0]*rlo, rhi+r, l)
	return math.FMA(poly, r3, l) + h
}

// fdlibm log1p coefficients, as glibc's s_log1p.c spells them.
const (
	l1pLn2hi = 6.93147180369123816490e-01
	l1pLn2lo = 1.90821492927058770002e-10
	lp1      = 6.666666666666735130e-01
	lp2      = 3.999999999940941908e-01
	lp3      = 2.857142874366239149e-01
	lp4      = 2.222219843214978396e-01
	lp5      = 1.818357216161805012e-01
	lp6      = 1.531383769920937332e-01
	lp7      = 1.479819860511658591e-01
	twoThird = 0.66666666666666666
)

// Log1p is glibc's __log1p_fma (fdlibm s_log1p.c compiled with FMA): the
// branches and fused operations follow its machine code, not the C text, where
// the two disagree (k = 0 for -0x1.2bec4p-2 high words is decided as compiled).
func Log1p(x float64) float64 {
	bits := math.Float64bits(x)
	hx := uint32(bits >> 32)
	ax := hx & 0x7fffffff
	var u, c float64
	var k int32
	switch {
	case int32(hx) < 0x3fda827a:
		if ax > 0x3fefffff { // x <= -1
			if x == -1 {
				return math.Inf(-1)
			}
			return math.NaN()
		}
		if ax <= 0x3e1fffff { // |x| < 2**-29
			if ax <= 0x3c8fffff {
				return x
			}
			return math.FMA(-(x * x), 0.5, x)
		}
		if hx+0x402d413c > 0x402d413c { // -0.2929 < x < 0.41422: f = x, k = 0
			return log1pPoly(x, float64(float64(x*0.5)*x), 0, 0)
		}
		u, k, c = log1pU(x)
	case int32(hx) > 0x7fefffff:
		return x + x
	case int32(hx) <= 0x433fffff:
		u, k, c = log1pU(x)
	default: // x >= 2**53: u = x, no correction term
		u, k, c = x, int32(hx>>20)-0x3ff, 0
	}
	uhi := uint32(math.Float64bits(u) >> 32)
	hu := uhi & 0xfffff
	ulo := math.Float64bits(u) & 0xffffffff
	if hu > 0x6a09d { // normalize u/2
		u = math.Float64frombits(uint64(hu|0x3fe00000)<<32 | ulo)
		k++
		hu = (0x100000 - hu) >> 2
	} else { // normalize u
		u = math.Float64frombits(uint64(hu|0x3ff00000)<<32 | ulo)
	}
	f := u - 1.0
	hfsq := float64(float64(f*0.5) * f)
	if hu != 0 {
		return log1pPoly(f, hfsq, k, c)
	}
	kd := float64(k)
	if f == 0 {
		if k == 0 {
			return 0
		}
		return math.FMA(kd, l1pLn2hi, math.FMA(kd, l1pLn2lo, c))
	}
	r := float64(math.FMA(-f, twoThird, 1.0) * hfsq)
	if k == 0 {
		return f - r
	}
	t := r - math.FMA(kd, l1pLn2lo, c) - f
	return math.FMA(kd, l1pLn2hi, -t)
}

// log1pU: u = 1 + x, its exponent k and the correction term c / u.
func log1pU(x float64) (u float64, k int32, c float64) {
	u = x + 1.0
	k = int32(uint32(math.Float64bits(u)>>32)>>20) - 0x3ff
	if k <= 0 {
		c = x - (u - 1.0)
	} else {
		c = 1.0 - (u - x)
	}
	return u, k, c / u
}

func log1pPoly(f, hfsq float64, k int32, c float64) float64 {
	s := f / (f + 2.0)
	z := s * s
	r2 := math.FMA(z, lp3, lp2)
	r3 := math.FMA(z, lp5, lp4)
	r4 := math.FMA(z, lp7, lp6)
	z2 := z * z
	z4 := z2 * z2
	z6 := z4 * z2
	r := math.FMA(z6, r4, math.FMA(z4, r3, math.FMA(z, lp1, float64(z2*r2))))
	w := float64((r + hfsq) * s)
	if k == 0 {
		return f - (hfsq - w)
	}
	kd := float64(k)
	t := hfsq - (math.FMA(kd, l1pLn2lo, c) + w) - f
	return math.FMA(kd, l1pLn2hi, -t)
}
