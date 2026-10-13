package dwg

// Reed-Solomon(255,239) FEC decoder for R2004+ system sections (spec §4.1;
// R2007 §5.13). DWG interleaves ~239 bytes of payload with ~16 parity bytes
// per 255-byte chunk; when CRC-8 fails, the parity allows recovery of up to
// 8 corrupted bytes per chunk.
//
// GF(256) over the AES primitive polynomial 0x11D, generator α = 2. Usage
// in this package is defensive: the decode path is only reached when a
// system page / codeword does not verify.

// Reed-Solomon (255, 239) parameters.
const (
	RSParityBytes   = 16
	RSCodewordBytes = 255
	RSMessageBytes  = RSCodewordBytes - RSParityBytes
)

type gfTables struct {
	exp [256]byte
	log [256]byte
}

var gfTablesCache = func() *gfTables {
	t := &gfTables{}
	x := uint16(1)
	for i := 0; i < 255; i++ {
		t.exp[i] = byte(x)
		t.log[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11D
		}
	}
	return t
}()

func (t *gfTables) mul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	s := uint16(t.log[a]) + uint16(t.log[b])
	return t.exp[s%255]
}

func (t *gfTables) inv(a byte) byte {
	return t.exp[(255-uint16(t.log[a]))%255]
}

func (t *gfTables) polyEval(poly []byte, x byte) byte {
	y := poly[0]
	for _, c := range poly[1:] {
		y = t.mul(y, x) ^ c
	}
	return y
}

// rssyndromes computes the 16 syndromes of a 255-byte codeword.
func rssyndromes(codeword []byte) [RSParityBytes]byte {
	t := gfTablesCache
	var s [RSParityBytes]byte
	for i := 0; i < RSParityBytes; i++ {
		alpha := t.exp[(i+1)%255]
		s[i] = t.polyEval(codeword, alpha)
	}
	return s
}

func rsBerlekampMassey(syn *[RSParityBytes]byte) []byte {
	t := gfTablesCache
	lambda := []byte{1}
	corr := []byte{1}
	for n := 0; n < RSParityBytes; n++ {
		delta := syn[n]
		for i := 1; i < len(lambda); i++ {
			delta ^= t.mul(lambda[i], syn[n-i])
		}
		if delta == 0 {
			corr = append([]byte{0}, corr...)
		} else if 2*(len(lambda)-1) <= n {
			prev := append([]byte(nil), lambda...)
			shifted := append([]byte{0}, corr...)
			newLambda := append([]byte(nil), lambda...)
			for i, c := range shifted {
				if i < len(newLambda) {
					newLambda[i] ^= t.mul(delta, c)
				} else {
					newLambda = append(newLambda, t.mul(delta, c))
				}
			}
			lambda = newLambda
			dInv := t.inv(delta)
			corr = make([]byte, len(prev))
			for i, c := range prev {
				corr[i] = t.mul(dInv, c)
			}
			corr = append([]byte{0}, corr...)
		} else {
			shifted := append([]byte{0}, corr...)
			for i, c := range shifted {
				if i < len(lambda) {
					lambda[i] ^= t.mul(delta, c)
				} else {
					lambda = append(lambda, t.mul(delta, c))
				}
			}
			corr = append([]byte{0}, corr...)
		}
	}
	return lambda
}

func rsChienSearch(lambda []byte) []int {
	t := gfTablesCache
	var roots []int
	for i := 0; i < 255; i++ {
		alphaI := t.exp[(255-i)%255]
		if t.polyEval(lambda, alphaI) == 0 {
			roots = append(roots, i)
		}
	}
	return roots
}

// RSVerify returns nil if the codeword has zero syndromes (intact) or can
// be corrected in place, or an error if decoding fails.
func RSVerify(codeword []byte) error {
	if len(codeword) != RSCodewordBytes {
		return newError(ErrSectionMap, "Reed-Solomon codeword must be %d bytes, got %d", RSCodewordBytes, len(codeword))
	}
	t := gfTablesCache
	syn := rssyndromes(codeword)
	allZero := true
	for _, b := range syn {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil
	}
	lambda := rsBerlekampMassey(&syn)
	roots := rsChienSearch(lambda)
	if len(roots) == 0 || len(roots) > RSParityBytes/2 {
		return newError(ErrSectionMap, "Reed-Solomon: %d error locations, max correctable is %d", len(roots), RSParityBytes/2)
	}
	n := len(roots)
	mat := make([][]byte, n)
	for i := range mat {
		mat[i] = make([]byte, n+1)
	}
	for i := 0; i < n; i++ {
		for j, pos := range roots {
			alphaPos := t.exp[pos%255]
			power := byte(1)
			for k := 0; k < i+1; k++ {
				power = t.mul(power, alphaPos)
			}
			mat[i][j] = power
		}
		mat[i][n] = syn[i]
	}
	for col := 0; col < n; col++ {
		pivot := col
		for pivot < n && mat[pivot][col] == 0 {
			pivot++
		}
		if pivot == n {
			return newError(ErrSectionMap, "Reed-Solomon: singular system")
		}
		if pivot != col {
			mat[col], mat[pivot] = mat[pivot], mat[col]
		}
		inv := t.inv(mat[col][col])
		for j := col; j <= n; j++ {
			mat[col][j] = t.mul(mat[col][j], inv)
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := mat[row][col]
			if factor == 0 {
				continue
			}
			for j := col; j <= n; j++ {
				mat[row][j] ^= t.mul(factor, mat[col][j])
			}
		}
	}
	for k, pos := range roots {
		magnitude := mat[k][n]
		idx := RSCodewordBytes - 1 - pos
		if idx < len(codeword) {
			codeword[idx] ^= magnitude
		}
	}
	synAfter := rssyndromes(codeword)
	for _, b := range synAfter {
		if b != 0 {
			return newError(ErrSectionMap, "Reed-Solomon: decode did not converge")
		}
	}
	return nil
}
