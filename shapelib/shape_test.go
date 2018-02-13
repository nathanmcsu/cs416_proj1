package shapelib

import (
	"strconv"
	"testing"
)

// Test normalization
func TestNormalizeSvgString(t *testing.T) {
	svgString := "   M 10 10 L 5 , 5 h -3 Z"
	svgNorm := normalizeSvgString(svgString)
	svgExpected := "M10,10L5,5h-3Z"

	if svgNorm != svgExpected {
		t.Error("Expected "+svgExpected+", got ", svgNorm)
	}
}

// Test command parsing
func TestGetCommands(t *testing.T) {
	path := Shape{ShapeType: PATH, ShapeSvgString: "M 10 10 L 5 5 h -3 Z"}
	pathCommands, _ := path.getPathCommands()
	pathCommandsExpected := []PathCommand{
		PathCommand{"M", 10, 10},
		PathCommand{"L", 5, 5},
		PathCommand{"h", -3, 0},
		PathCommand{"Z", 0, 0}}

	for i := range pathCommands {
		svgCommand := pathCommands[i]
		pathCommandExpected := pathCommandsExpected[i]

		if svgCommand.CmdType != pathCommandExpected.CmdType {
			t.Error("Expected "+pathCommandExpected.CmdType+", got ", svgCommand.CmdType)
		}

		if svgCommand.X != pathCommandExpected.X {
			t.Error("Expected "+strconv.Itoa(int(pathCommandExpected.X))+", got ", strconv.Itoa(int(svgCommand.X)))
		}

		if svgCommand.Y != pathCommandExpected.Y {
			t.Error("Expected "+strconv.Itoa(int(pathCommandExpected.Y))+", got ", strconv.Itoa(int(svgCommand.Y)))
		}
	}

	circle := Shape{ShapeType: CIRCLE, ShapeSvgString: "X 10 Y 10 R 34"}
	circleCommands, _ := circle.getCircleCommands()
	circleCommandsExpected := []CircleCommand{
		CircleCommand{"X", 10},
		CircleCommand{"Y", 10},
		CircleCommand{"R", 34}}

	for i := range circleCommands {
		svgCommand := circleCommands[i]
		circleCommandExpected := circleCommandsExpected[i]

		if svgCommand.CmdType != circleCommandExpected.CmdType {
			t.Error("Expected "+circleCommandExpected.CmdType+", got ", svgCommand.CmdType)
		}

		if svgCommand.Val != circleCommandExpected.Val {
			t.Error("Expected "+strconv.Itoa(int(circleCommandExpected.Val))+", got ", strconv.Itoa(int(svgCommand.Val)))
		}
	}
}

// Test get geometry
func TestGetPathGeometry(t *testing.T) {
	shapeCircle1 := Shape{ShapeType: CIRCLE, ShapeSvgString: "X 10 Y 10 R 34"}
	shapeCircle2 := Shape{ShapeType: CIRCLE, ShapeSvgString: "M 10 10 X 10 Y 10 R 34"}
	shapeTransClosed := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 Z"}
	shapeTransOpen1 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3"}
	shapeTransOpen2 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 M 10 10 h 3 l -1 3"}
	shapeFilledClosed1 := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 Z"}
	shapeFilledClosed2 := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 L 10 10"}
	shapeFilledClosed3 := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 L 10 10 Z m 10 10 h 3 l -1 3 L 10 10 Z"}
	shapeFilledOpen := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 10 10 h 3 l -1 3"}

	if _, err := shapeTransClosed.GetGeometry(); err != nil {
		t.Error("Expected no error for transparent closed shape, got: ", err)
	}

	if _, err := shapeTransOpen1.GetGeometry(); err != nil {
		t.Error("Expected no error for transparent open shape, got: ", err)
	}

	if _, err := shapeTransOpen2.GetGeometry(); err != nil {
		t.Error("Expected no error for transparent open shape with multiple 'moveto', got: ", err)
	}

	if _, err := shapeFilledClosed1.GetGeometry(); err != nil {
		t.Error("Expected no error for filled close shape, got: ", err)
	}

	if _, err := shapeFilledClosed2.GetGeometry(); err != nil {
		t.Error("Expected no error for filled close shape, got: ", err)
	}

	if _, err := shapeFilledClosed3.GetGeometry(); err == nil {
		t.Error("Expected error for filled closed shape with multiple 'moveto', but got none.")
	}

	if _, err := shapeFilledOpen.GetGeometry(); err == nil {
		t.Error("Expected error for filled open shape, got none")
	}

	if _, err := shapeCircle1.GetGeometry(); err != nil {
		t.Error("Expected no error for circle, got ", err)
	}

	if _, err := shapeCircle2.GetGeometry(); err == nil {
		t.Error("Expected error for circle, got none")
	}
}

// Test vertices generated from commands
func TestGetVertices(t *testing.T) {
	shapeClosed := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 Z"}
	shapeOpen := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3"}
	_geoClosed, _ := shapeClosed.GetGeometry()
	_geoOpen, _ := shapeOpen.GetGeometry()
	geoClosed, _ := interface{}(_geoClosed).(PathGeometry)
	geoOpen, _ := interface{}(_geoOpen).(PathGeometry)

	vertices := geoClosed.VertexSets[0]
	verticesExpected := []Point{
		Point{10, 10},
		Point{13, 10},
		Point{12, 13},
		Point{10, 10}}
	for i := range vertices {
		vertex := vertices[i]
		vertexExpected := verticesExpected[i]

		if vertex.X != vertexExpected.X {
			t.Error("Expected "+strconv.Itoa(int(vertexExpected.X))+", got ", strconv.Itoa(int(vertex.X)))
		}

		if vertex.Y != vertexExpected.Y {
			t.Error("Expected "+strconv.Itoa(int(vertexExpected.Y))+", got ", strconv.Itoa(int(vertex.Y)))
		}
	}

	vertices = geoOpen.VertexSets[0]
	verticesExpected = []Point{
		Point{10, 10},
		Point{13, 10},
		Point{12, 13}}
	for i := range vertices {
		vertex := vertices[i]
		vertexExpected := verticesExpected[i]

		if vertex.X != vertexExpected.X {
			t.Error("Expected "+strconv.Itoa(int(vertexExpected.X))+", got ", strconv.Itoa(int(vertex.X)))
		}

		if vertex.Y != vertexExpected.Y {
			t.Error("Expected "+strconv.Itoa(int(vertexExpected.Y))+", got ", strconv.Itoa(int(vertex.Y)))
		}
	}
}

// Test line segments generated from vertices
func TestGetLineSegments(t *testing.T) {
	shapeClosed := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3 Z"}
	shapeOpen := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 h 3 l -1 3"}
	_geoClosed, _ := shapeClosed.GetGeometry()
	_geoOpen, _ := shapeOpen.GetGeometry()
	geoClosed, _ := interface{}(_geoClosed).(PathGeometry)
	geoOpen, _ := interface{}(_geoOpen).(PathGeometry)

	lineSegments := getLineSegments(geoClosed.VertexSets[0])
	lineSegmentsExpected := []LineSegment{
		LineSegment{Start: Point{10, 10}, End: Point{13, 10}, A: 0, B: -3, C: -30},
		LineSegment{Start: Point{13, 10}, End: Point{12, 13}, A: 3, B: 1, C: 49},
		LineSegment{Start: Point{12, 13}, End: Point{10, 10}, A: -3, B: 2, C: -10}}
	for i := range lineSegments {
		lineSegment := lineSegments[i]
		lineSegmentExpected := lineSegmentsExpected[i]

		if lineSegment.Start != lineSegmentExpected.Start {
			t.Error("Start point mismatch on line segment.")
		}

		if lineSegment.End != lineSegmentExpected.End {
			t.Error("End point mismatch on line segment.")
		}

		if lineSegment.A != lineSegmentExpected.A {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.A))+", got ", strconv.Itoa(int(lineSegment.A)))
		}

		if lineSegment.B != lineSegmentExpected.B {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.B))+", got ", strconv.Itoa(int(lineSegment.B)))
		}

		if lineSegment.C != lineSegmentExpected.C {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.C))+", got ", strconv.Itoa(int(lineSegment.C)))
		}
	}

	lineSegments = getLineSegments(geoOpen.VertexSets[0])
	lineSegmentsExpected = []LineSegment{
		LineSegment{Start: Point{10, 10}, End: Point{13, 10}, A: 0, B: -3, C: -30},
		LineSegment{Start: Point{13, 10}, End: Point{12, 13}, A: 3, B: 1, C: 49}}
	for i := range lineSegments {
		lineSegment := lineSegments[i]
		lineSegmentExpected := lineSegmentsExpected[i]

		if lineSegment.Start != lineSegmentExpected.Start {
			t.Error("Start point mismatch on line segment.")
		}

		if lineSegment.End != lineSegmentExpected.End {
			t.Error("End point mismatch on line segment.")
		}

		if lineSegment.A != lineSegmentExpected.A {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.A))+", got ", strconv.Itoa(int(lineSegment.A)))
		}

		if lineSegment.B != lineSegmentExpected.B {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.B))+", got ", strconv.Itoa(int(lineSegment.B)))
		}

		if lineSegment.C != lineSegmentExpected.C {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.C))+", got ", strconv.Itoa(int(lineSegment.C)))
		}
	}
}

// Test line-to-line overlap
func TestLineOverlap(t *testing.T) {
	shape1 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 L 5 5"}
	shape2 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 5 5 L 10 10 Z"}
	shape3 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 7 5 L 5 10 v -2 Z"}
	_geo1, _ := shape1.GetGeometry()
	_geo2, _ := shape2.GetGeometry()
	_geo3, _ := shape3.GetGeometry()
	geo1, _ := interface{}(_geo1).(PathGeometry)
	geo2, _ := interface{}(_geo2).(PathGeometry)
	geo3, _ := interface{}(_geo3).(PathGeometry)

	lineSegments1 := getLineSegments(geo1.VertexSets[0])
	lineSegments2 := getLineSegments(geo2.VertexSets[0])
	lineSegments3 := getLineSegments(geo3.VertexSets[0])

	// Test parallel lines
	if lineSegments1[0].Intersects(lineSegments2[0]) != true {
		t.Error("Expected true, got false")
	}

	if lineSegments1[0].Intersects(lineSegments2[1]) != true {
		t.Error("Expected true, got false")
	}

	// Test non-parallel lines
	if lineSegments1[0].Intersects(lineSegments3[0]) != true {
		t.Error("Expected true, got false")
	}

	if lineSegments1[0].Intersects(lineSegments3[2]) != true {
		t.Error("Expected true, got false")
	}

	if lineSegments1[0].Intersects(lineSegments3[1]) != false {
		t.Error("Expected false, got true")
	}
}

// Test shape validity
func TestShapeValid(t *testing.T) {
	xMax := uint32(100)
	yMax := uint32(100)

	shapeLineInBound := Shape{ShapeType: PATH, Stroke: "non-transparent", Fill: "transparent", ShapeSvgString: "M 10 10 L 5 5 "}
	shapeOutOfMinBound := Shape{ShapeType: PATH, Stroke: "non-transparent", Fill: "transparent", ShapeSvgString: "M 5 5 h -7"}
	shapeOutOfMaxBound := Shape{ShapeType: PATH, Stroke: "non-transparent", Fill: "transparent", ShapeSvgString: "M 7 5 h 10000000"}
	shapeSelfIntersectTrans := Shape{ShapeType: PATH, Stroke: "non-transparent", Fill: "transparent", ShapeSvgString: "M 5 5 L 10 10 h -5 L 10 5 Z"}
	shapeSelfIntersectNonTrans := Shape{ShapeType: PATH, Stroke: "non-transparent", Fill: "non-transparent", ShapeSvgString: "M 5 5 L 10 10 h -5 L 10 5 Z"}
	shapeCircleInBound := Shape{ShapeType: CIRCLE, Stroke: "non-transparent", Fill: "non-transparent", ShapeSvgString: "X 10 Y 10 R 5"}
	shapeCircleOutOfBound := Shape{ShapeType: CIRCLE, Stroke: "non-transparent", Fill: "non-transparent", ShapeSvgString: "X 100 Y 100 R 1500"}

	if valid, _, err := shapeLineInBound.IsValid(xMax, yMax); valid != true {
		t.Error("Expected valid shape, got", err)
	}

	if valid, _, err := shapeSelfIntersectTrans.IsValid(xMax, yMax); valid != true {
		t.Error("Expected valid shape, got", err)
	}

	if valid, _, err := shapeOutOfMinBound.IsValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}

	if valid, _, err := shapeOutOfMaxBound.IsValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}

	if valid, _, err := shapeSelfIntersectNonTrans.IsValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}

	if valid, _, err := shapeCircleInBound.IsValid(xMax, yMax); valid != true {
		t.Error("Expected valid shape, got", err)
	}

	if valid, _, err := shapeCircleOutOfBound.IsValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}
}

// Test ink usage
func TestInkRequired(t *testing.T) {
	shape1 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 L 5 5"}                                // Line
	shape2 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 5 5 L 10 10 h -5 L 10 5 Z"}                  // Twisted Square
	shape3 := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 5 5 h 5 v 5 h -5 Z"}                     // Square
	shape4 := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 5 5 h 4 l -2 5 z"}                       // Triangle
	shape5 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 5 L 26 5 l -4 15 l -4 -10 l -4 10 Z"}     // Dracula teeth
	shape6 := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 10 5 L 26 5 l -4 15 l -4 -10 l -4 10 Z"} // Dracula teeth
	shape7 := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 10 l 5 5 M 20 20 l 5 5"}                  // Muliple moveto
	geo1, _ := shape1.GetGeometry()
	geo2, _ := shape2.GetGeometry()
	geo3, _ := shape3.GetGeometry()
	geo4, _ := shape4.GetGeometry()
	geo5, _ := shape5.GetGeometry()
	geo6, _ := shape6.GetGeometry()
	geo7, _ := shape7.GetGeometry()

	if ink := geo1.GetInkCost(); ink != 8 {
		t.Error("Expected 8 ink units, got", strconv.FormatUint(ink, 10))
	}

	if ink := geo2.GetInkCost(); ink != 26 {
		t.Error("Expected 26 ink units, got", strconv.FormatUint(ink, 10))
	}

	// Note: although its 5X5 at first glance, its actual 5X6 in pixels
	if ink := geo3.GetInkCost(); ink != 30 {
		t.Error("Expected 30 ink units, got", strconv.FormatUint(ink, 10))
	}

	if ink := geo4.GetInkCost(); ink != 12 {
		t.Error("Expected 12 ink units, got", strconv.FormatUint(ink, 10))
	}

	if ink := geo5.GetInkCost(); ink != 70 {
		t.Error("Expected 70 ink units, got", strconv.FormatUint(ink, 10))
	}

	if ink := geo6.GetInkCost(); ink != 156 {
		t.Error("Expected 156 ink units, got", strconv.FormatUint(ink, 10))
	}

	if ink := geo7.GetInkCost(); ink != 16 {
		t.Error("Expected 8 ink units, got", strconv.FormatUint(ink, 10))
	}
}

// Test overlap
func TestOverlap(t *testing.T) {
	shapeTriangle := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 5 5 h 4 l -2 5 z"}                                // Triangle -- Transparent
	shapeTriangleFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 5 5 h 4 l -2 5 z"}                      // Triangle -- Filled
	shapeDracula := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 10 5 L 26 5 l -4 15 l -4 -10 l -4 10 Z"}           // Dracula teeth -- Transparent
	shapeDraculaFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 10 5 L 26 5 l -4 15 l -4 -10 l -4 10 Z"} // Dracula teeth -- Filled
	geoTriangle, _ := shapeTriangle.GetGeometry()
	geoTriangleFilled, _ := shapeTriangleFilled.GetGeometry()
	geoDracula, _ := shapeDracula.GetGeometry()
	geoDraculaFilled, _ := shapeDraculaFilled.GetGeometry()

	// Test polygon surrounding shape
	shapeSquare := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 1 1 H 40 V -40 H -40 Z"}
	geoSquare, _ := shapeSquare.GetGeometry()
	if overlap := geoTriangle.HasOverlap(geoSquare); overlap != false {
		t.Error("Expected big non-filled square not to overlap smaller triangle.")
	}

	squareFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 1 1 h 40 v 40 h -40 Z"}
	geoSquareFilled, _ := squareFilled.GetGeometry()
	if overlap := geoTriangle.HasOverlap(geoSquareFilled); overlap != true {
		t.Error("Expected big filled square to overlap smaller triangle.")
	}

	// Test basic intersection
	shapeTrans := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 5 5 v 3 h 10 v -5 Z"}
	shapeFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 5 5 v 3 h 10 v -5 Z"}
	shapeMulti := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 5 5 v 3 h 10 v -5 Z M 5 5 v -3 h 10 v -5 Z"}
	geoTrans, _ := shapeTrans.GetGeometry()
	geoFilled, _ := shapeFilled.GetGeometry()
	geoMulti, _ := shapeMulti.GetGeometry()

	overlap := geoTriangle.HasOverlap(geoTrans)
	overlap = geoTriangleFilled.HasOverlap(geoTrans)
	overlap = geoTriangle.HasOverlap(geoFilled)
	overlap = geoTriangleFilled.HasOverlap(geoFilled)
	overlap = geoTriangle.HasOverlap(geoMulti)
	overlap = geoTriangleFilled.HasOverlap(geoMulti)

	if overlap != true {
		t.Error("Expected overlap, got no overlap.")
	}

	// Test cases with shapes within weird polygons
	longRectangle := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 15 12 h 1 v 1 h -1 Z"}
	longRectangleFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 15 12 h 1 v 1 h -1 Z"}
	squareCenter := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 18 6 h 1 v 1 h -1 Z"}
	squareCenterFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 18 6 h 1 v 1 h -1 Z"}
	squareLeftTooth := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 14 12 h 1 v 1 h -1 Z"}
	squareLeftToothFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 14 12 h 1 v 1 h -1 Z"}
	squareBetweenTeeth := Shape{ShapeType: PATH, Fill: "transparent", ShapeSvgString: "M 19 19 h 1 v -1 h -1 Z"}
	squareBetweenTeethFilled := Shape{ShapeType: PATH, Fill: "non-transparent", ShapeSvgString: "M 19 19 h 1 v -1 h -1 Z"}
	geoLongRectangle, _ := longRectangle.GetGeometry()
	geoLongRectangleFilled, _ := longRectangleFilled.GetGeometry()
	geoSquareCenter, _ := squareCenter.GetGeometry()
	geoSquareCenterFilled, _ := squareCenterFilled.GetGeometry()
	geoSquareLeftTooth, _ := squareLeftTooth.GetGeometry()
	geoSquareLeftToothFilled, _ := squareLeftToothFilled.GetGeometry()
	geoSquareBetweenTeeth, _ := squareBetweenTeeth.GetGeometry()
	geoSquareBetweenTeethFilled, _ := squareBetweenTeethFilled.GetGeometry()

	if overlap := geoDracula.HasOverlap(geoLongRectangle); overlap != true {
		t.Error("Expected long rectangle across dracula teeth to overlap, got no overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoLongRectangle); overlap != true {
		t.Error("Expected long rectangle across dracula teeth to overlap, got no overlap.")
	}
	if overlap := geoDracula.HasOverlap(geoLongRectangleFilled); overlap != true {
		t.Error("Expected long rectangle across dracula teeth to overlap, got no overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoLongRectangleFilled); overlap != true {
		t.Error("Expected long rectangle across dracula teeth to overlap, got no overlap.")
	}

	if overlap := geoDracula.HasOverlap(geoSquareCenter); overlap != false {
		t.Error("Expected small square in center of dracula teeth to not overlap, got overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoSquareCenter); overlap != true {
		t.Error("Expected small square in center of dracula teeth to overlap, got no overlap.")
	}
	if overlap := geoDracula.HasOverlap(geoSquareCenterFilled); overlap != false {
		t.Error("Expected small square in center of dracula teeth to not overlap, got overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoSquareCenterFilled); overlap != true {
		t.Error("Expected small square in center of dracula teeth to overlap, got no overlap.")
	}

	if overlap := geoDracula.HasOverlap(geoSquareLeftTooth); overlap != false {
		t.Error("Expected left tooth square to not overlap, got overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoSquareLeftTooth); overlap != true {
		t.Error("Expected left tooth square to overlap, got no overlap.")
	}
	if overlap := geoDracula.HasOverlap(geoSquareLeftToothFilled); overlap != false {
		t.Error("Expected left tooth square to not overlap, got overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoSquareLeftToothFilled); overlap != true {
		t.Error("Expected left tooth square to overlap, got no overlap.")
	}

	if overlap := geoDracula.HasOverlap(geoSquareBetweenTeeth); overlap != false {
		t.Error("Expected square between teeth (outside draculas teeth polygon) to not overlap, got overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoSquareBetweenTeeth); overlap != false {
		t.Error("Expected square between teeth (outside draculas teeth polygon) to not overlap, got overlap.")
	}
	if overlap := geoDracula.HasOverlap(geoSquareBetweenTeethFilled); overlap != false {
		t.Error("Expected square between teeth (outside draculas teeth polygon) to not overlap, got overlap.")
	}
	if overlap := geoDraculaFilled.HasOverlap(geoSquareBetweenTeethFilled); overlap != false {
		t.Error("Expected square between teeth (outside draculas teeth polygon) to not overlap, got overlap.")
	}

}
