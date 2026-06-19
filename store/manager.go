package store

import (
	"fmt"
	"math"

	"github.com/billmakes/bsptile/v2/common"

	log "github.com/sirupsen/logrus"
)

const (
	SplitVertical   = "vertical"
	SplitHorizontal = "horizontal"

	Stacked uint8 = 1
	Ordered uint8 = 2
	Visible uint8 = 3
)

type Manager struct {
	Name       string    // Manager name with window clients
	Location   *Location // Manager workspace and screen location
	Decoration bool      // Window decoration is enabled
	Root       *Node     `json:"-"` // Runtime BSP tree
}

type Node struct {
	Parent *Node           `json:"-"`
	First  *Node           `json:"-"`
	Second *Node           `json:"-"`
	Client *Client         `json:"-"`
	Split  string          `json:"-"`
	Ratio  float64         `json:"-"`
	Bounds common.Geometry `json:"-"`
}

type Location struct {
	Desktop uint // Location desktop index
	Screen  uint // Location screen index
}

type Directions struct {
	Top    bool // Indicates proportion changes on the top
	Right  bool // Indicates proportion changes on the right
	Bottom bool // Indicates proportion changes on the bottom
	Left   bool // Indicates proportion changes on the left
}

type Clients struct {
	Maximum int       // Maximum visible clients
	Stacked []*Client `json:"-"` // Stored window clients
}

func CreateManager(loc Location) *Manager {
	return CreateBSPManager(loc)
}

func CreateBSPManager(loc Location) *Manager {
	return &Manager{
		Name:       fmt.Sprintf("manager-%d-%d", loc.Desktop, loc.Screen),
		Location:   &loc,
		Decoration: common.Config.WindowDecoration,
	}
}

func (mg *Manager) EnableDecoration()  { mg.Decoration = true }
func (mg *Manager) DisableDecoration() { mg.Decoration = false }
func (mg *Manager) DecorationEnabled() bool {
	return mg.Decoration
}
func (mg *Manager) DecorationDisabled() bool {
	return !mg.Decoration
}

func (n *Node) leaf() bool {
	return n != nil && n.Client != nil
}

func (mg *Manager) AddClient(c *Client) {
	if c == nil || mg.node(c) != nil {
		return
	}
	log.Debug("Add client to BSP tree [", c.Latest.Class, ", ", mg.Name, "]")

	leaf := &Node{Client: c, Ratio: 0.5}
	if mg.Root == nil {
		mg.Root = leaf
		return
	}

	target := mg.activeNode()
	if target == nil {
		target = mg.lastLeaf(mg.Root)
	}

	parent := &Node{
		Parent: target.Parent,
		Split:  splitForBounds(target.Bounds),
		Ratio:  0.5,
	}
	parent.First, parent.Second = target, leaf
	parent.First.Parent = parent
	parent.Second.Parent = parent

	if parent.Parent == nil {
		mg.Root = parent
	} else if parent.Parent.First == target {
		parent.Parent.First = parent
	} else {
		parent.Parent.Second = parent
	}
}

func (mg *Manager) RemoveClient(c *Client) {
	leaf := mg.node(c)
	if leaf == nil {
		return
	}
	log.Debug("Remove client from BSP tree [", c.Latest.Class, ", ", mg.Name, "]")

	if leaf.Parent == nil {
		mg.Root = nil
		return
	}

	parent := leaf.Parent
	sibling := parent.First
	if sibling == leaf {
		sibling = parent.Second
	}
	sibling.Parent = parent.Parent

	if parent.Parent == nil {
		mg.Root = sibling
	} else if parent.Parent.First == parent {
		parent.Parent.First = sibling
	} else {
		parent.Parent.Second = sibling
	}
}

func (mg *Manager) SwapClient(c1 *Client, c2 *Client) {
	n1, n2 := mg.node(c1), mg.node(c2)
	if n1 == nil || n2 == nil || n1 == n2 {
		return
	}
	log.Info("Swap BSP leaves [", c1.Latest.Class, "-", c2.Latest.Class, ", ", mg.Name, "]")
	n1.Client, n2.Client = n2.Client, n1.Client
}

func (mg *Manager) InsertClient(source, target *Client, edge string) {
	if source == nil || target == nil || source == target {
		return
	}
	var split string
	sourceFirst := false
	switch edge {
	case "left":
		split, sourceFirst = SplitVertical, true
	case "right":
		split, sourceFirst = SplitVertical, false
	case "top":
		split, sourceFirst = SplitHorizontal, true
	case "bottom":
		split, sourceFirst = SplitHorizontal, false
	default:
		return
	}
	if mg.node(target) == nil {
		return
	}
	if mg.node(source) != nil {
		mg.RemoveClient(source)
	}
	targetLeaf := mg.node(target)
	if targetLeaf == nil {
		return
	}
	log.Info("Insert BSP leaf [", source.Latest.Class, " ", edge, " of ", target.Latest.Class, ", ", mg.Name, "]")
	sourceLeaf := &Node{Client: source, Ratio: 0.5}
	parent := &Node{Parent: targetLeaf.Parent, Split: split, Ratio: 0.5}
	if sourceFirst {
		parent.First, parent.Second = sourceLeaf, targetLeaf
	} else {
		parent.First, parent.Second = targetLeaf, sourceLeaf
	}
	parent.First.Parent = parent
	parent.Second.Parent = parent
	if parent.Parent == nil {
		mg.Root = parent
	} else if parent.Parent.First == targetLeaf {
		parent.Parent.First = parent
	} else {
		parent.Parent.Second = parent
	}
}

func (mg *Manager) ActiveClient() *Client {
	if node := mg.activeNode(); node != nil {
		return node.Client
	}
	return nil
}

func (mg *Manager) NextClient() *Client {
	return mg.cycleClient(1)
}

func (mg *Manager) PreviousClient() *Client {
	return mg.cycleClient(-1)
}

func (mg *Manager) cycleClient(delta int) *Client {
	clients := mg.Clients(Stacked)
	if len(clients) == 0 {
		return nil
	}
	for i, c := range clients {
		if c.Window.Id == Windows.Active.Id {
			index := (i + delta + len(clients)) % len(clients)
			return clients[index]
		}
	}
	return nil
}

func (mg *Manager) DirectionClient(direction common.Direction) *Client {
	active := mg.ActiveClient()
	if active == nil {
		return nil
	}
	activeGeometry := mg.clientGeometry(active)

	var target *Client
	var best common.DirectionScore

	for _, c := range mg.Clients(Stacked) {
		if c == active {
			continue
		}
		score, ok := common.ScoreDirection(activeGeometry, mg.clientGeometry(c), direction)
		if !ok {
			continue
		}
		if target == nil || common.BetterDirectionScore(score, best) {
			target = c
			best = score
		}
	}
	return target
}

func (mg *Manager) clientGeometry(c *Client) common.Geometry {
	if node := mg.node(c); node != nil && node.Bounds.Width > 0 && node.Bounds.Height > 0 {
		return node.Bounds
	}
	return c.Latest.Dimensions.Geometry
}

func (mg *Manager) Reset() {
	mg.resetNode(mg.Root)
}

func (mg *Manager) Balance() {
	mg.balanceNode(mg.Root)
}

func (mg *Manager) Rotate() {
	mg.rotateNode(mg.Root)
}

func (mg *Manager) balanceNode(node *Node) int {
	if node == nil {
		return 0
	}
	if node.leaf() {
		return 1
	}
	first := mg.balanceNode(node.First)
	second := mg.balanceNode(node.Second)
	total := first + second
	if total > 0 {
		node.Ratio = float64(first) / float64(total)
	}
	return total
}

func (mg *Manager) rotateNode(node *Node) {
	if node == nil || node.leaf() {
		return
	}

	// Clockwise rotation maps left/right to top/bottom. Mapping top/bottom
	// to left/right reverses child order and therefore the split ratio.
	if node.Split == SplitVertical {
		node.Split = SplitHorizontal
	} else {
		node.Split = SplitVertical
		node.First, node.Second = node.Second, node.First
		node.Ratio = 1.0 - node.Ratio
	}

	mg.rotateNode(node.First)
	mg.rotateNode(node.Second)
}

func (mg *Manager) resetNode(node *Node) {
	if node == nil || node.leaf() {
		return
	}
	node.Ratio = 0.5
	mg.resetNode(node.First)
	mg.resetNode(node.Second)
}

func (mg *Manager) Apply(geom common.Geometry) {
	mg.applyNode(mg.Root, geom)
}

func (mg *Manager) applyNode(node *Node, geom common.Geometry) {
	if node == nil {
		return
	}
	node.Bounds = geom
	if node.leaf() {
		gap := common.Config.WindowGapSize
		w, h := common.MaxInt(geom.Width-gap, 1), common.MaxInt(geom.Height-gap, 1)
		x, y := geom.X+gap/2, geom.Y+gap/2
		node.Client.Limit(w, h)
		node.Client.MoveWindow(x, y, w, h)
		node.Client.Latest.Dimensions.Geometry = common.Geometry{X: x, Y: y, Width: w, Height: h}
		// MoveWindow calls c.Update() which reads geometry from X. Because the
		// window manager processes moves asynchronously, the X readback may
		// return the pre-move position and set Location.Screen to the old screen.
		// Override it from the manager's authoritative location.
		node.Client.Latest.Location.Screen = mg.Location.Screen
		return
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
	mg.applyNode(node.First, first)
	mg.applyNode(node.Second, second)
}

func (mg *Manager) IncreaseProportion() {
	mg.resizeActive(common.Config.ProportionStep)
}

func (mg *Manager) DecreaseProportion() {
	mg.resizeActive(-common.Config.ProportionStep)
}

// ResizeDirection nudges the active window in direction d so that *something*
// moves that way. It first tries to grow the named edge outward; if that edge
// is already at the screen border (no matching ancestor), it falls back to
// shrinking the opposite edge in the same direction. With this, "resize_right"
// always reacts: either the right edge extends rightward, or the left edge is
// pulled rightward against a fixed right edge.
func (mg *Manager) ResizeDirection(d common.Direction) bool {
	if mg.GrowDirection(d, true) {
		return true
	}
	return mg.GrowDirection(common.Opposite(d), false)
}

// GrowDirection moves the active leaf's edge on the named side outward
// (when grow is true) or inward (when grow is false). It walks up the BSP
// tree to find the first ancestor split whose boundary IS that edge — i.e.
// a vertical split where the active leaf's subtree sits on the First half
// for the right edge, or on the Second half for the left edge (and the
// equivalent for top/bottom on horizontal splits). Returns false when the
// edge is at the screen border (no matching ancestor) or the clamp blocks
// further movement.
func (mg *Manager) GrowDirection(d common.Direction, grow bool) bool {
	leaf := mg.activeNode()
	if leaf == nil {
		return false
	}

	split := SplitVertical
	if d == common.Up || d == common.Down {
		split = SplitHorizontal
	}

	// Right and Down edges live on the First side of their split (the boundary
	// is "after" the First child). Left and Up edges live on the Second side.
	edgeFirst := d == common.Right || d == common.Down

	delta := common.Config.ProportionStep
	// Growing the First-side edge means pushing the boundary outward —
	// increasing the ratio. Growing a Second-side edge means decreasing.
	// Shrinking is the opposite.
	if (edgeFirst && !grow) || (!edgeFirst && grow) {
		delta = -delta
	}

	child := leaf
	for node := leaf.Parent; node != nil; node = node.Parent {
		if node.Split == split && (node.First == child) == edgeFirst {
			ratio := clampRatio(node.Ratio + delta)
			if ratio == node.Ratio {
				return false
			}
			node.Ratio = ratio
			return true
		}
		child = node
	}

	return false
}

func (mg *Manager) DirectionProportion(direction common.Direction) bool {
	leaf := mg.activeNode()
	if leaf == nil {
		return false
	}

	split := SplitVertical
	delta := common.Config.ProportionStep
	switch direction {
	case common.Left:
		delta = -delta
	case common.Right:
	case common.Up:
		split = SplitHorizontal
		delta = -delta
	case common.Down:
		split = SplitHorizontal
	default:
		return false
	}

	for node := leaf.Parent; node != nil; node = node.Parent {
		if node.Split != split {
			continue
		}
		ratio := clampRatio(node.Ratio + delta)
		if ratio == node.Ratio {
			return false
		}
		node.Ratio = ratio
		return true
	}

	return false
}

func (mg *Manager) resizeActive(delta float64) {
	leaf := mg.activeNode()
	if leaf == nil || leaf.Parent == nil {
		return
	}
	parent := leaf.Parent
	if parent.Second == leaf {
		delta = -delta
	}
	parent.Ratio = clampRatio(parent.Ratio + delta)
}

func (mg *Manager) UpdateProportions(c *Client, d *Directions, geom common.Geometry) {
	leaf := mg.node(c)
	if leaf == nil {
		return
	}

	gap := common.Config.WindowGapSize / 2
	if d.Left {
		mg.resizeBoundary(leaf, SplitVertical, false, geom.X-gap)
	}
	if d.Right {
		mg.resizeBoundary(leaf, SplitVertical, true, geom.X+geom.Width+gap)
	}
	if d.Top {
		mg.resizeBoundary(leaf, SplitHorizontal, false, geom.Y-gap)
	}
	if d.Bottom {
		mg.resizeBoundary(leaf, SplitHorizontal, true, geom.Y+geom.Height+gap)
	}
}

func (mg *Manager) resizeBoundary(leaf *Node, split string, trailing bool, boundary int) {
	for node := leaf.Parent; node != nil; node = node.Parent {
		if node.Split != split {
			continue
		}

		inFirst := contains(node.First, leaf)
		// The trailing edge of the first subtree and leading edge of the
		// second subtree are the only edges shared across this split.
		if (trailing && !inFirst) || (!trailing && inFirst) {
			continue
		}

		origin, size := node.Bounds.X, node.Bounds.Width
		if split == SplitHorizontal {
			origin, size = node.Bounds.Y, node.Bounds.Height
		}
		if size <= 0 {
			return
		}

		node.Ratio = clampRatio(float64(boundary-origin) / float64(size))
		return
	}
}

func clampRatio(ratio float64) float64 {
	minimum := common.Config.ProportionMin
	if minimum <= 0 || minimum >= 0.5 {
		minimum = 0.1
	}
	return math.Min(math.Max(ratio, minimum), 1.0-minimum)
}

func (mg *Manager) Clients(flag uint8) []*Client {
	clients := make([]*Client, 0)
	mg.collect(mg.Root, &clients)
	if flag == Ordered {
		return mg.ordered(clients)
	}
	return clients
}

func (mg *Manager) collect(node *Node, clients *[]*Client) {
	if node == nil {
		return
	}
	if node.leaf() {
		*clients = append(*clients, node.Client)
		return
	}
	mg.collect(node.First, clients)
	mg.collect(node.Second, clients)
}

func (mg *Manager) ordered(clients []*Client) []*Client {
	ordered := make([]*Client, 0, len(clients))
	for _, w := range Windows.Stacked {
		for _, c := range clients {
			if w.Id == c.Window.Id {
				ordered = append(ordered, c)
				break
			}
		}
	}
	return ordered
}

func (mg *Manager) node(c *Client) *Node {
	if c == nil {
		return nil
	}
	return findNode(mg.Root, c)
}

func findNode(node *Node, c *Client) *Node {
	if node == nil {
		return nil
	}
	if node.leaf() {
		if node.Client.Window.Id == c.Window.Id {
			return node
		}
		return nil
	}
	if found := findNode(node.First, c); found != nil {
		return found
	}
	return findNode(node.Second, c)
}

func contains(node, target *Node) bool {
	if node == nil {
		return false
	}
	if node == target {
		return true
	}
	return contains(node.First, target) || contains(node.Second, target)
}

func (mg *Manager) activeNode() *Node {
	for _, c := range mg.Clients(Stacked) {
		if c.Window.Id == Windows.Active.Id {
			return mg.node(c)
		}
	}
	return nil
}

func (mg *Manager) lastLeaf(node *Node) *Node {
	if node == nil || node.leaf() {
		return node
	}
	return mg.lastLeaf(node.Second)
}

func splitForBounds(bounds common.Geometry) string {
	if bounds.Height > bounds.Width {
		return SplitHorizontal
	}
	return SplitVertical
}
