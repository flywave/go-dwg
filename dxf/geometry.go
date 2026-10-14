package dxf

// Point2 / Point3 are the coordinate workhorses. Z defaults to 0 for 2D
// entities; writers omit nothing — group triples always emit all three
// components, matching what AutoCAD writes.

// Point2 is a 2D coordinate (LWPOLYLINE vertices, fit points, …).
type Point2 struct {
	X, Y float64
}

// Point3 is a 3D coordinate.
type Point3 struct {
	X, Y, Z float64
}

// Pt2 lifts a 2D point to 3D at z=0.
func Pt2(x, y float64) Point3 { return Point3{X: x, Y: y} }

// Pt3 builds a 3D point.
func Pt3(x, y, z float64) Point3 { return Point3{X: x, Y: y, Z: z} }
