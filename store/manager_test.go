package store

import (
	"math"
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

func TestFloatingClassification(t *testing.T) {
	previousIgnore := common.Config.WindowIgnore
	common.Config.WindowIgnore = [][]string{{"ignored.*", ""}}
	t.Cleanup(func() {
		common.Config.WindowIgnore = previousIgnore
	})

	tests := []struct {
		name string
		info *Info
		want bool
	}{
		{
			name: "dialog",
			info: &Info{Class: "Xfce4-settings-manager", Types: []string{"_NET_WM_WINDOW_TYPE_DIALOG"}},
			want: true,
		},
		{
			name: "configured ignored normal window",
			info: &Info{Class: "ignored-app", Types: []string{"_NET_WM_WINDOW_TYPE_NORMAL"}},
			want: true,
		},
		{
			name: "managed normal window",
			info: &Info{Class: "terminal", Types: []string{"_NET_WM_WINDOW_TYPE_NORMAL"}},
			want: false,
		},
		{
			name: "dock",
			info: &Info{Class: "panel", Types: []string{"_NET_WM_WINDOW_TYPE_DOCK"}},
			want: false,
		},
		{
			name: "notification",
			info: &Info{Class: "notify", Types: []string{"_NET_WM_WINDOW_TYPE_NOTIFICATION"}},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsFloating(test.info); got != test.want {
				t.Fatalf("IsFloating() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNaturalRestoreGeometryKeepsLatestCenterAndOriginalSize(t *testing.T) {
	client := &Client{
		Original: &Info{Dimensions: Dimensions{Geometry: common.Geometry{
			X: 10, Y: 20, Width: 320, Height: 180,
		}}},
		Latest: &Info{Dimensions: Dimensions{Geometry: common.Geometry{
			X: 100, Y: 100, Width: 800, Height: 600,
		}}},
	}

	got := client.RestoreGeometry(Natural)
	want := common.Geometry{X: 340, Y: 310, Width: 320, Height: 180}
	if got != want {
		t.Fatalf("natural restore geometry = %+v, want %+v", got, want)
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

func TestBSPInsertInvalidRequestPreservesSource(t *testing.T) {
	mg := CreateBSPManager(Location{})
	first := testClient(1)
	second := testClient(2)
	mg.AddClient(first)
	Windows = &XWindows{Active: *first.Window}
	mg.AddClient(second)

	mg.InsertClient(second, first, "invalid")

	if mg.node(first) == nil || mg.node(second) == nil {
		t.Fatal("invalid insert removed an existing client")
	}
	if len(mg.Clients(Stacked)) != 2 {
		t.Fatalf("client count = %d, want 2", len(mg.Clients(Stacked)))
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

func TestBSPBalanceProducesEqualLeafAreas(t *testing.T) {
	// Unbalanced tree: root vertically splits into one leaf (left) and a
	// horizontally split subtree of two leaves (right).
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

	mg.Balance()

	// Left leaf carries 1 of 3 windows, so the root split should give it 1/3.
	if math.Abs(root.Ratio-1.0/3.0) > 1e-9 {
		t.Fatalf("root ratio = %v, want 1/3", root.Ratio)
	}
	// The right subtree's children each carry one window, so a half split.
	if math.Abs(right.Ratio-0.5) > 1e-9 {
		t.Fatalf("nested ratio = %v, want 0.5", right.Ratio)
	}

	// Walk the tree the same way applyNode does and check every leaf ends up
	// with the same area. Done inline to avoid Apply()'s X-server side effects.
	leafBounds := walkBounds(root, common.Geometry{Width: 900, Height: 900})
	if len(leafBounds) != 3 {
		t.Fatalf("expected 3 leaves, got %d", len(leafBounds))
	}
	expected := leafBounds[0].Width * leafBounds[0].Height
	for i, b := range leafBounds {
		if b.Width*b.Height != expected {
			t.Fatalf("leaf %d area = %d, want %d", i, b.Width*b.Height, expected)
		}
	}
}

func walkBounds(node *Node, geom common.Geometry) []common.Geometry {
	if node == nil {
		return nil
	}
	if node.leaf() {
		return []common.Geometry{geom}
	}
	first, second := geom, geom
	if node.Split == SplitVertical {
		size := int(math.Round(float64(geom.Width) * node.Ratio))
		first.Width = size
		second.X += size
		second.Width -= size
	} else {
		size := int(math.Round(float64(geom.Height) * node.Ratio))
		first.Height = size
		second.Y += size
		second.Height -= size
	}
	return append(walkBounds(node.First, first), walkBounds(node.Second, second)...)
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

func TestBSPResizeDirectionFallsBackToShrinkAtScreenEdge(t *testing.T) {
	previousStep := common.Config.ProportionStep
	previousMin := common.Config.ProportionMin
	common.Config.ProportionStep = 0.1
	common.Config.ProportionMin = 0.1
	t.Cleanup(func() {
		common.Config.ProportionStep = previousStep
		common.Config.ProportionMin = previousMin
	})

	// Tree:
	//        root (V, 0.5)
	//        /   \
	//      left   right (H, 0.5)
	//             /   \
	//          topR   bottomR
	//
	// Active = bottomR. Its right and bottom edges are at the screen border;
	// its left edge is the root V split; its top edge is the inner H split.
	left := &Node{Client: testClient(1)}
	topRight := &Node{Client: testClient(2)}
	bottomRight := &Node{Client: testClient(3)}
	right := &Node{First: topRight, Second: bottomRight, Split: SplitHorizontal, Ratio: 0.5}
	root := &Node{First: left, Second: right, Split: SplitVertical, Ratio: 0.5}
	left.Parent = root
	right.Parent = root
	topRight.Parent = right
	bottomRight.Parent = right
	mg := &Manager{Root: root}
	Windows = &XWindows{Active: *bottomRight.Client.Window}

	// resize_left: grow_left works (decrease root) since bottomR's left edge
	// IS the root V split boundary.
	if !mg.ResizeDirection(common.Left) || root.Ratio != 0.4 {
		t.Fatalf("resize_left: root ratio = %v, want 0.4 (grow_left path)", root.Ratio)
	}

	// resize_right: grow_right has no matching ancestor (right edge is the
	// screen), so it falls back to shrink_left — moving bottomR's left edge
	// rightward — which means INCREASING the root ratio.
	rootBefore := root.Ratio
	if !mg.ResizeDirection(common.Right) {
		t.Fatal("resize_right should fall back to shrink at screen edge")
	}
	if root.Ratio != rootBefore+0.1 {
		t.Fatalf("resize_right fallback: root ratio = %v, want %v", root.Ratio, rootBefore+0.1)
	}

	// resize_down: same fallback story on the H axis. Fallback shrinks
	// bottomR's top edge downward by INCREASING the inner H ratio.
	rightBefore := right.Ratio
	if !mg.ResizeDirection(common.Down) {
		t.Fatal("resize_down should fall back to shrink at screen edge")
	}
	if right.Ratio != rightBefore+0.1 {
		t.Fatalf("resize_down fallback: right ratio = %v, want %v", right.Ratio, rightBefore+0.1)
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

func TestDirectionClientPrefersNearbyDiagonalOverFarAligned(t *testing.T) {
	active := testClient(1)
	nearDiagonal := testClient(2)
	farAligned := testClient(3)
	active.Latest.Dimensions.Geometry = common.Geometry{X: 1000, Y: 500, Width: 400, Height: 400}
	nearDiagonal.Latest.Dimensions.Geometry = common.Geometry{X: 500, Y: 100, Width: 400, Height: 300}
	farAligned.Latest.Dimensions.Geometry = common.Geometry{X: -1000, Y: 500, Width: 400, Height: 400}

	activeNode := &Node{Client: active}
	nearNode := &Node{Client: nearDiagonal}
	farNode := &Node{Client: farAligned}
	left := &Node{First: farNode, Second: nearNode}
	root := &Node{First: left, Second: activeNode}
	farNode.Parent = left
	nearNode.Parent = left
	left.Parent = root
	activeNode.Parent = root

	mg := &Manager{Root: root}
	Windows = &XWindows{Active: *active.Window}

	if target := mg.DirectionClient(common.Left); target != nearDiagonal {
		t.Fatalf("left target = %v, want nearby diagonal window", target)
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
