package dwg

// Shared helpers for ACIS-backed modeler entities (3DSOLID, BODY, REGION) —
// spec §20.4.41. Autodesk stores parametric solids as an opaque ACIS SAT/SAB
// byte stream wrapped in a thin DWG envelope.

// MaxSATBlobBytes caps the accumulated SAT payload (32 MiB).
const MaxSATBlobBytes = 32 * 1024 * 1024

// MaxModelerElements caps the §20.4.41 wireframe block counts.
const MaxModelerElements = 65_536

// RevisionGUIDBytes is the R2018+ width of the data-store revision field.
//
// Measured: R2018 records carry 16 bytes (a valid RFC-4122 v4 GUID, per the
// reference); every R2013 record of `OpenRL_PointLight.dwg` closes its data
// fields exactly on its boundary with **8** bytes instead. See revisionBytes.
const RevisionGUIDBytes = 16

// revisionBytes is the on-wire width of the R2013+ data-store revision field.
func revisionBytes(version Version) int {
	if version == R2018 {
		return 16
	}
	return 8
}

// SatBlob is the ACIS envelope data decoded from the stream.
type SatBlob struct {
	Empty   bool
	Version uint16
	Bytes   []byte
}

// ModelerTail is the §20.4.41 tail every ACIS entity writes after its
// envelope.
type ModelerTail struct {
	WireframePresent bool
	Point            *Point3D
	NumIsolines      int32
	IsolinesPresent  bool
	NumWires         int32
	NumSilhouettes   int32
	AcisEmpty2       bool
	RevisionGUID     []byte
}

// ModelerRecord is one whole ACIS record: envelope plus tail.
type ModelerRecord struct {
	Blob        SatBlob
	InDataStore bool
	Tail        ModelerTail
}

func modelerBoundedCount(value int32, what string) (int32, error) {
	if value < 0 || value > MaxModelerElements {
		return 0, newError(ErrSectionMap, "ACIS %s is %d, outside 0..=%d", what, value, MaxModelerElements)
	}
	return value, nil
}

func modelerStepWire(c *BitCursor) error {
	if _, err := c.ReadRC(); err != nil {
		return err
	}
	if _, err := c.ReadBL(); err != nil {
		return err
	}
	if _, err := c.ReadBS(); err != nil {
		return err
	}
	if _, err := c.ReadBL(); err != nil {
		return err
	}
	bl, err := c.ReadBL()
	if err != nil {
		return err
	}
	pointCount, err := modelerBoundedCount(bl, "wire point count")
	if err != nil {
		return err
	}
	for i := int32(0); i < pointCount; i++ {
		if _, err := ReadBD3(c); err != nil {
			return err
		}
	}
	transform, err := c.ReadB()
	if err != nil {
		return err
	}
	if transform {
		for i := 0; i < 4; i++ {
			if _, err := ReadBD3(c); err != nil {
				return err
			}
		}
		if _, err := c.ReadBD(); err != nil {
			return err
		}
		for i := 0; i < 3; i++ {
			if _, err := c.ReadB(); err != nil {
				return err
			}
		}
	}
	return nil
}

// readSatBlocks reads the version-1 block loop of an ACIS envelope.
func readSatBlocks(c *BitCursor, version uint16) ([]byte, error) {
	if version != 1 {
		return nil, newError(ErrSectionMap, "ACIS envelope reports version %d; only version 1 can be stepped", version)
	}
	var blob []byte
	for {
		bl, err := c.ReadBL()
		if err != nil {
			return nil, err
		}
		if bl == 0 {
			return blob, nil
		}
		if bl < 0 {
			return nil, newError(ErrSectionMap, "SAT block size is negative (%d)", bl)
		}
		blockSize := int(bl)
		remainingBytes := c.RemainingBits() / 8
		if blockSize > remainingBytes {
			return nil, newError(ErrSectionMap, "SAT block size (%d) exceeds remaining cursor bytes (%d)", blockSize, remainingBytes)
		}
		if len(blob)+blockSize > MaxSATBlobBytes {
			return nil, newError(ErrSectionMap, "SAT blob exceeds %d-byte cap", MaxSATBlobBytes)
		}
		for i := 0; i < blockSize; i++ {
			b, err := c.ReadRC()
			if err != nil {
				return nil, err
			}
			blob = append(blob, b)
		}
	}
}

func readSatBlob(c *BitCursor) (SatBlob, error) {
	empty, err := c.ReadB()
	if err != nil {
		return SatBlob{}, err
	}
	if empty {
		return SatBlob{Empty: true}, nil
	}
	if _, err := c.ReadB(); err != nil {
		return SatBlob{}, err
	}
	version, err := c.ReadBSU()
	if err != nil {
		return SatBlob{}, err
	}
	blob, err := readSatBlocks(c, version)
	if err != nil {
		return SatBlob{}, err
	}
	return SatBlob{Version: version, Bytes: blob}, nil
}

// decodeModelerRecord reads the §20.4.41 record body.
func decodeModelerRecord(c *BitCursor, version Version, inDataStore bool) (ModelerRecord, error) {
	var blob SatBlob
	if inDataStore {
		blob = SatBlob{Empty: true}
	} else {
		var err error
		blob, err = readSatBlob(c)
		if err != nil {
			return ModelerRecord{}, err
		}
	}
	var tail ModelerTail
	wireframePresent, err := c.ReadB()
	if err != nil {
		return ModelerRecord{}, err
	}
	tail.WireframePresent = wireframePresent
	if wireframePresent {
		pointPresent, err := c.ReadB()
		if err != nil {
			return ModelerRecord{}, err
		}
		if pointPresent {
			p, err := ReadBD3(c)
			if err != nil {
				return ModelerRecord{}, err
			}
			tail.Point = &p
		}
		iso, err := c.ReadBL()
		if err != nil {
			return ModelerRecord{}, err
		}
		if tail.NumIsolines, err = modelerBoundedCount(iso, "isoline count"); err != nil {
			return ModelerRecord{}, err
		}
		if tail.IsolinesPresent, err = c.ReadB(); err != nil {
			return ModelerRecord{}, err
		}
		if tail.IsolinesPresent {
			nw, err := c.ReadBL()
			if err != nil {
				return ModelerRecord{}, err
			}
			if tail.NumWires, err = modelerBoundedCount(nw, "wire count"); err != nil {
				return ModelerRecord{}, err
			}
			for i := int32(0); i < tail.NumWires; i++ {
				if err := modelerStepWire(c); err != nil {
					return ModelerRecord{}, err
				}
			}
			ns, err := c.ReadBL()
			if err != nil {
				return ModelerRecord{}, err
			}
			if tail.NumSilhouettes, err = modelerBoundedCount(ns, "silhouette count"); err != nil {
				return ModelerRecord{}, err
			}
			for i := int32(0); i < tail.NumSilhouettes; i++ {
				if _, err := c.ReadBL(); err != nil { // viewport id
					return ModelerRecord{}, err
				}
				for j := 0; j < 3; j++ {
					if _, err := ReadBD3(c); err != nil {
						return ModelerRecord{}, err
					}
				}
				if _, err := c.ReadB(); err != nil { // perspective
					return ModelerRecord{}, err
				}
				wires, err := c.ReadBL()
				if err != nil {
					return ModelerRecord{}, err
				}
				wc, err := modelerBoundedCount(wires, "silhouette wire count")
				if err != nil {
					return ModelerRecord{}, err
				}
				for j := int32(0); j < wc; j++ {
					if err := modelerStepWire(c); err != nil {
						return ModelerRecord{}, err
					}
				}
			}
		}
	}
	if tail.AcisEmpty2, err = c.ReadB(); err != nil {
		return ModelerRecord{}, err
	}
	if !tail.AcisEmpty2 {
		if _, err := c.ReadB(); err != nil {
			return ModelerRecord{}, err
		}
		version2, err := c.ReadBSU()
		if err != nil {
			return ModelerRecord{}, err
		}
		if _, err := readSatBlocks(c, version2); err != nil {
			return ModelerRecord{}, err
		}
	}
	if version.IsR2007Plus() {
		if _, err := c.ReadBL(); err != nil {
			return ModelerRecord{}, err
		}
	}
	if inDataStore {
		if _, err := c.ReadB(); err != nil {
			return ModelerRecord{}, err
		}
		for i := 0; i < 3; i++ {
			if _, err := c.ReadBB(); err != nil {
				return ModelerRecord{}, err
			}
		}
		guid := make([]byte, revisionBytes(version))
		for i := range guid {
			b, err := c.ReadRC()
			if err != nil {
				return ModelerRecord{}, err
			}
			guid[i] = b
		}
		if _, err := c.ReadBL(); err != nil {
			return ModelerRecord{}, err
		}
		tail.RevisionGUID = guid
	}
	return ModelerRecord{Blob: blob, InDataStore: inDataStore, Tail: tail}, nil
}
