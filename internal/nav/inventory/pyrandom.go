package inventory

import "math"

// pyRandom is the part of Python's random.Random that selftest --sample uses:
// the MT19937 generator seeded from an integer, getrandbits for widths up to
// 32, _randbelow and sample over a list, giving the same picks as CPython.
type pyRandom struct {
	mt  [624]uint32
	idx int
}

// newPyRandom is random.Random(seed) for 0 <= seed < 2**32: CPython seeds with
// init_by_array over the 32-bit words of abs(seed) (one word for 0).
func newPyRandom(seed uint32) *pyRandom {
	r := &pyRandom{}
	r.initByArray([]uint32{seed})
	return r
}

func (r *pyRandom) initGenrand(s uint32) {
	r.mt[0] = s
	for i := 1; i < 624; i++ {
		r.mt[i] = 1812433253*(r.mt[i-1]^(r.mt[i-1]>>30)) + uint32(i)
	}
	r.idx = 624
}

func (r *pyRandom) initByArray(key []uint32) {
	r.initGenrand(19650218)
	i, j := 1, 0
	k := max(624, len(key))
	for ; k > 0; k-- {
		r.mt[i] = (r.mt[i] ^ ((r.mt[i-1] ^ (r.mt[i-1] >> 30)) * 1664525)) + key[j] + uint32(j)
		i++
		j++
		if i >= 624 {
			r.mt[0] = r.mt[623]
			i = 1
		}
		if j >= len(key) {
			j = 0
		}
	}
	for k = 623; k > 0; k-- {
		r.mt[i] = (r.mt[i] ^ ((r.mt[i-1] ^ (r.mt[i-1] >> 30)) * 1566083941)) - uint32(i)
		i++
		if i >= 624 {
			r.mt[0] = r.mt[623]
			i = 1
		}
	}
	r.mt[0] = 0x80000000
}

// uint32 is genrand_uint32.
func (r *pyRandom) uint32() uint32 {
	const upper, lower = 0x80000000, 0x7fffffff
	if r.idx >= 624 {
		for kk := 0; kk < 624; kk++ {
			y := (r.mt[kk] & upper) | (r.mt[(kk+1)%624] & lower)
			v := r.mt[(kk+397)%624] ^ (y >> 1)
			if y&1 != 0 {
				v ^= 0x9908b0df
			}
			r.mt[kk] = v
		}
		r.idx = 0
	}
	y := r.mt[r.idx]
	r.idx++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}

// randbelow is _randbelow_with_getrandbits(n) for 0 < n < 2**32.
func (r *pyRandom) randbelow(n int) int {
	k := 0
	for v := n; v > 0; v >>= 1 {
		k++
	}
	for {
		v := int(r.uint32() >> (32 - k))
		if v < n {
			return v
		}
	}
}

// sample is Random.sample(population, k) for 0 <= k <= len(population): the
// pool method for small populations, else the set method.
func sample[T any](r *pyRandom, population []T, k int) []T {
	n := len(population)
	result := make([]T, k)
	setsize := 21
	if k > 5 {
		setsize += int(math.Pow(4, math.Ceil(math.Log(float64(k*3))/math.Log(4))))
	}
	if n <= setsize {
		pool := append([]T(nil), population...)
		for i := 0; i < k; i++ {
			j := r.randbelow(n - i)
			result[i] = pool[j]
			pool[j] = pool[n-i-1]
		}
		return result
	}
	selected := map[int]bool{}
	for i := 0; i < k; i++ {
		j := r.randbelow(n)
		for selected[j] {
			j = r.randbelow(n)
		}
		selected[j] = true
		result[i] = population[j]
	}
	return result
}
