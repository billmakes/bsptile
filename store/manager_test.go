package store

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/billmakes/bsptile/v2/common"
)

func testClient(id xproto.Window) *Client {
	return &Client{
		Window: &XWindow{Id: id},
		Latest: &Info{Class: "test"},
	}
}

func TestBSPInsertSplitsLongestSide(t *testing.T) {
	mg := CreateBSPManager(Location{})
	first := testClient(1)
	second := testClient(2)
	third := testClient(3)

	mg.AddClient(first)
	mg.Root.Bounds = common.Geometry{Width: 100, Height: 300}
	Windows = &XWindows{Active: *first.Window}
	mg.AddClient(second)
	mg.node(second).Bounds = common.Geometry{Width: 400, Height: 100}
	Windows.Active = *second.Window
	mg.AddClient(third)

	if mg.Root.Split != SplitHorizontal {
		t.Fatalf("root split = %q, want horizontal", mg.Root.Split)
	}
	if mg.Root.Second.Split != SplitVertical {
		t.Fatalf("nested split = %q, want vertical", mg.Root.Second.Split)
	}
	if len(mg.Clients(Stacked)) != 3 {
		t.Fatalf("client count = %d, want 3", len(mg.Clients(Stacked)))
	}
}

func TestBSPRemoveCollapsesParent(t *testing.T) {
	mg := CreateBSPManager(Location{})
	first := testClient(1)
	second := testClient(2)

	mg.AddClient(first)
	Windows = &XWindows{Active: *first.Window}
	mg.AddClient(second)
	mg.RemoveClient(second)

	if mg.Root == nil || mg.Root.Client != first {
		t.Fatal("remaining leaf was not promoted to root")
	}
}

func TestBSPSwapKeepsTreeShape(t *testing.T) {
	mg := CreateBSPManager(Location{})
	first := testClient(1)
	second := testClient(2)

	mg.AddClient(first)
	Windows = &XWindows{Active: *first.Window}
	mg.AddClient(second)
	root := mg.Root

	mg.SwapClient(first, second)

	if mg.Root != root || mg.Root.First.Client != second || mg.Root.Second.Client != first {
		t.Fatal("swap changed structure or failed to exchange leaves")
	}
}

func TestBSPRotateClockwise(t *testing.T) {
	firefox := &Node{Client: testClient(1)}
	a := &Node{Client: testClient(2)}
	b := &Node{Client: testClient(3)}
	right := &Node{
		First:  a,
		Second: b,
		Split:  SplitHorizontal,
		Ratio:  0.4,
	}
	root := &Node{
		First:  firefox,
		Second: right,
		Split:  SplitVertical,
		Ratio:  0.6,
	}
	firefox.Parent = root
	right.Parent = root
	a.Parent = right
	b.Parent = right
	mg := &Manager{Root: root}

	mg.Rotate()

	if root.Split != SplitHorizontal || root.First != firefox || root.Second != right {
		t.Fatal("first rotation did not move the left subtree to the top")
	}
	if right.Split != SplitVertical || right.First != b || right.Second != a || right.Ratio != 0.6 {
		t.Fatal("first rotation did not rotate the nested subtree clockwise")
	}

	mg.Rotate()

	if root.Split != SplitVertical || root.First != right || root.Second != firefox || root.Ratio != 0.4 {
		t.Fatal("second rotation did not move the top subtree to the right")
	}
	if right.Split != SplitHorizontal || right.First != b || right.Second != a {
		t.Fatal("second rotation changed nested child order incorrectly")
	}
}

func TestBSPResizeUpdatesSharedBoundaries(t *testing.T) {
	previousMin := common.Config.ProportionMin
	previousGap := common.Config.WindowGapSize
	common.Config.ProportionMin = 0.1
	common.Config.WindowGapSize = 0
	t.Cleanup(func() {
		common.Config.ProportionMin = previousMin
		common.Config.WindowGapSize = previousGap
	})

	firefox := &Node{Client: testClient(1)}
	a := &Node{Client: testClient(2)}
	b := &Node{Client: testClient(3)}
	right := &Node{
		First:  a,
		Second: b,
		Split:  SplitHorizontal,
		Ratio:  0.5,
		Bounds: common.Geometry{X: 500, Width: 500, Height: 800},
	}
	root := &Node{
		First:  firefox,
		Second: right,
		Split:  SplitVertical,
		Ratio:  0.5,
		Bounds: common.Geometry{Width: 1000, Height: 800},
	}
	firefox.Parent = root
	right.Parent = root
	a.Parent = right
	b.Parent = right
	mg := &Manager{Root: root}

	mg.UpdateProportions(firefox.Client, &Directions{Right: true}, common.Geometry{Width: 600, Height: 800})
	if root.Ratio != 0.6 {
		t.Fatalf("root ratio = %v, want 0.6", root.Ratio)
	}

	mg.UpdateProportions(a.Client, &Directions{Bottom: true}, common.Geometry{X: 600, Y: 0, Width: 400, Height: 480})
	if right.Ratio != 0.6 {
		t.Fatalf("nested ratio = %v, want 0.6", right.Ratio)
	}

	root.Ratio = 0.6
	mg.UpdateProportions(a.Client, &Directions{Right: true}, common.Geometry{X: 600, Width: 400, Height: 480})
	if root.Ratio != 0.6 {
		t.Fatalf("screen-edge resize changed root ratio to %v", root.Ratio)
	}
}

func TestBSPDirectionProportionUsesNearestMatchingAncestor(t *testing.T) {
	previousStep := common.Config.ProportionStep
	previousMin := common.Config.ProportionMin
	common.Config.ProportionStep = 0.1
	common.Config.ProportionMin = 0.1
	t.Cleanup(func() {
		common.Config.ProportionStep = previousStep
		common.Config.ProportionMin = previousMin
	})

	left := &Node{Client: testClient(1)}
	topRight := &Node{Client: testClient(2)}
	bottomRight := &Node{Client: testClient(3)}
	right := &Node{
		First:  topRight,
		Second: bottomRight,
		Split:  SplitHorizontal,
		Ratio:  0.5,
	}
	root := &Node{
		First:  left,
		Second: right,
		Split:  SplitVertical,
		Ratio:  0.5,
	}
	left.Parent = root
	right.Parent = root
	topRight.Parent = right
	bottomRight.Parent = right
	mg := &Manager{Root: root}
	Windows = &XWindows{Active: *bottomRight.Client.Window}

	if !mg.DirectionProportion(common.Left) || root.Ratio != 0.4 {
		t.Fatalf("left resize ratio = %v, want 0.4", root.Ratio)
	}
	if !mg.DirectionProportion(common.Right) || root.Ratio != 0.5 {
		t.Fatalf("right resize ratio = %v, want 0.5", root.Ratio)
	}
	if !mg.DirectionProportion(common.Up) || right.Ratio != 0.4 {
		t.Fatalf("up resize ratio = %v, want 0.4", right.Ratio)
	}
	if !mg.DirectionProportion(common.Down) || right.Ratio != 0.5 {
		t.Fatalf("down resize ratio = %v, want 0.5", right.Ratio)
	}
}

func TestDirectionClientPrioritizesDistanceInRequestedDirection(t *testing.T) {
	active := testClient(1)
	nearLeft := testClient(2)
	farAlignedLeft := testClient(3)
	active.Latest.Dimensions.Geometry = common.Geometry{X: 2000, Y: 400, Width: 400, Height: 400}
	nearLeft.Latest.Dimensions.Geometry = common.Geometry{X: 1400, Y: 200, Width: 400, Height: 400}
	farAlignedLeft.Latest.Dimensions.Geometry = common.Geometry{X: 0, Y: 400, Width: 400, Height: 400}

	activeNode := &Node{Client: active}
	nearNode := &Node{Client: nearLeft}
	farNode := &Node{Client: farAlignedLeft}
	left := &Node{First: farNode, Second: nearNode}
	root := &Node{First: left, Second: activeNode}
	farNode.Parent = left
	nearNode.Parent = left
	left.Parent = root
	activeNode.Parent = root

	mg := &Manager{Root: root}
	Windows = &XWindows{Active: *active.Window}

	if target := mg.DirectionClient(common.Left); target != nearLeft {
		t.Fatalf("left target = %v, want nearest window", target)
	}
}

func TestDirectionClientPrefersPerpendicularOverlap(t *testing.T) {
	active := testClient(1)
	directAbove := testClient(2)
	closerDiagonal := testClient(3)
	active.Latest.Dimensions.Geometry = common.Geometry{X: 1000, Y: 500, Width: 400, Height: 400}
	directAbove.Latest.Dimensions.Geometry = common.Geometry{X: 1000, Y: 0, Width: 400, Height: 400}
	closerDiagonal.Latest.Dimensions.Geometry = common.Geometry{X: 500, Y: 350, Width: 400, Height: 100}

	activeNode := &Node{Client: active}
	directNode := &Node{Client: directAbove}
	diagonalNode := &Node{Client: closerDiagonal}
	above := &Node{First: directNode, Second: diagonalNode}
	root := &Node{First: above, Second: activeNode}
	directNode.Parent = above
	diagonalNode.Parent = above
	above.Parent = root
	activeNode.Parent = root

	mg := &Manager{Root: root}
	Windows = &XWindows{Active: *active.Window}

	if target := mg.DirectionClient(common.Up); target != directAbove {
		t.Fatalf("up target = %v, want directly overlapping window", target)
	}
}

func TestDirectionClientUsesSynchronousLeafBoundsAfterSwap(t *testing.T) {
	left := &Node{
		Client: testClient(1),
		Bounds: common.Geometry{X: 0, Width: 500, Height: 500},
	}
	right := &Node{
		Client: testClient(2),
		Bounds: common.Geometry{X: 500, Width: 500, Height: 500},
	}
	root := &Node{First: left, Second: right}
	left.Parent = root
	right.Parent = root
	mg := &Manager{Root: root}
	Windows = &XWindows{Active: *left.Client.Window}

	active := left.Client
	target := right.Client
	mg.SwapClient(active, target)

	// Client geometry is intentionally stale. Tree bounds must still place the
	// active client in the right leaf after the synchronous swap.
	active.Latest.Dimensions.Geometry = common.Geometry{X: 0, Width: 500, Height: 500}
	target.Latest.Dimensions.Geometry = common.Geometry{X: 500, Width: 500, Height: 500}

	if selected := mg.DirectionClient(common.Left); selected != target {
		t.Fatalf("left target = %v, want client in left BSP leaf", selected)
	}
}
