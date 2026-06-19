package store

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/jezek/xgb/xproto"

	"github.com/jezek/xgbutil/ewmh"
	"github.com/jezek/xgbutil/icccm"
	"github.com/jezek/xgbutil/motif"
	"github.com/jezek/xgbutil/xprop"
	"github.com/jezek/xgbutil/xrect"
	"github.com/jezek/xgbutil/xwindow"

	"github.com/billmakes/bsptile/v2/common"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	Window   *XWindow // X window object
	Original *Info    `json:"-"` // Original client window information
	Cached   *Info    `json:"-"` // Cached client window information
	Latest   *Info    // Latest client window information
	Locked   bool     // Internal client move/resize lock
}

type Info struct {
	Class      string     // Client window application name
	Name       string     // Client window title name
	Types      []string   // Client window types
	States     []string   // Client window states
	Location   Location   // Client window location
	Dimensions Dimensions // Client window dimensions
}

type Dimensions struct {
	Geometry   common.Geometry   // Client window geometry
	Hints      Hints             // Client window dimension hints
	Extents    ewmh.FrameExtents // Client window geometry extents
	AdjPos     bool              // Position adjustments on move/resize
	AdjSize    bool              // Size adjustments on move/resize
	AdjRestore bool              // Disable adjustments on restore
}

type Hints struct {
	Normal icccm.NormalHints // Client window geometry hints
	Motif  motif.Hints       // Client window decoration hints
}

const (
	Original uint8 = 1 // Flag to restore original info
	Latest   uint8 = 2 // Flag to restore latest info
	Natural  uint8 = 3 // Flag to restore latest position with original size
)

func CreateClient(w xproto.Window) *Client {
	return &Client{
		Window:   CreateXWindow(w),
		Original: GetInfo(w),
		Cached:   GetInfo(w),
		Latest:   GetInfo(w),
		Locked:   false,
	}
}

func (c *Client) Lock() {
	c.Locked = true
}

func (c *Client) UnLock() {
	c.Locked = false
}

func (c *Client) Limit(w, h int) bool {
	if !Compatible("icccm.SizeHintPMinSize") {
		return false
	}

	// Decoration extents
	ext := c.Latest.Dimensions.Extents
	dw, dh := ext.Left+ext.Right, ext.Top+ext.Bottom

	// Set window size limits
	nhints := c.Cached.Dimensions.Hints.Normal
	nhints.Flags |= icccm.SizeHintPMinSize
	nhints.MinWidth = uint(w - dw)
	nhints.MinHeight = uint(h - dh)
	icccm.WmNormalHintsSet(X, c.Window.Id, &nhints)

	return true
}

func (c *Client) UnLimit() bool {
	if !Compatible("icccm.SizeHintPMinSize") {
		return false
	}

	// Restore window size limits
	icccm.WmNormalHintsSet(X, c.Window.Id, &c.Cached.Dimensions.Hints.Normal)

	return true
}

func (c *Client) Decorate() bool {
	if motif.Decor(&c.Latest.Dimensions.Hints.Motif) || !motif.Decor(&c.Original.Dimensions.Hints.Motif) {
		return false
	}

	// Add window decorations
	mhints := c.Cached.Dimensions.Hints.Motif
	mhints.Flags |= motif.HintDecorations
	mhints.Decoration = motif.DecorationAll
	motif.WmHintsSet(X, c.Window.Id, &mhints)

	return true
}

func (c *Client) UnDecorate() bool {
	if !motif.Decor(&c.Latest.Dimensions.Hints.Motif) && motif.Decor(&c.Original.Dimensions.Hints.Motif) {
		return false
	}

	// Remove window decorations
	mhints := c.Cached.Dimensions.Hints.Motif
	mhints.Flags |= motif.HintDecorations
	mhints.Decoration = motif.DecorationNone
	motif.WmHintsSet(X, c.Window.Id, &mhints)

	return true
}

func (c *Client) Fullscreen() bool {
	if IsFullscreen(c.Latest) {
		return false
	}
	return RequestWindowState(c.Window.Id, ewmh.StateAdd, "_NET_WM_STATE_FULLSCREEN")
}

func (c *Client) UnFullscreen() bool {
	if !IsFullscreen(c.Latest) {
		return false
	}

	return RequestWindowState(c.Window.Id, ewmh.StateRemove, "_NET_WM_STATE_FULLSCREEN")
}

func (c *Client) UnMaximize() bool {
	if !IsMaximized(c.Latest) {
		return false
	}

	return RequestWindowState(c.Window.Id, ewmh.StateRemove,
		"_NET_WM_STATE_MAXIMIZED_VERT", "_NET_WM_STATE_MAXIMIZED_HORZ")
}

func (c *Client) MoveToDesktop(desktop uint32) bool {
	if desktop == ^uint32(0) {
		if !RequestWindowState(c.Window.Id, ewmh.StateAdd, "_NET_WM_STATE_STICKY") {
			return false
		}
	}
	return SetWindowDesktop(c.Window.Id, desktop)
}

func (c *Client) MoveToScreen(screen uint32) bool {
	if !c.MoveToScreenDirect(screen) {
		return false
	}

	// Simulate tracker pointer press
	Pointer.Press()

	return true
}

func (c *Client) MoveToScreenDirect(screen uint32) bool {
	geom := Workplace.Displays.Screens[screen].Geometry

	// Calculate move to position
	_, _, w, h := c.OuterGeometry()
	x, y := common.MaxInt(geom.Center().X-w/2, geom.X+100), common.MaxInt(geom.Center().Y-h/2, geom.Y+100)

	// Move window
	return MoveXWindow(c.Window.Id, x, y)
}

func (c *Client) CenterOnScreen() bool {
	screen := c.Latest.Location.Screen
	if int(screen) >= len(Workplace.Displays.Screens) {
		log.Warn("Center: screen index out of range [", c.Latest.Class, " screen=", screen, "]")
		return false
	}
	geom := Workplace.Displays.Screens[screen].Geometry
	_, _, w, h := c.OuterGeometry()
	x := geom.Center().X - w/2
	y := geom.Center().Y - h/2
	log.Info("Center window [", c.Latest.Class, "] to ", x, ",", y, " (screen ", screen, " center ", geom.Center(), ", size ", w, "x", h, ")")
	return MoveXWindow(c.Window.Id, x, y)
}

func (c *Client) MoveWindow(x, y, w, h int) {
	if c.Locked {
		log.Info("Reject window move/resize [", c.Latest.Class, "]")

		// Remove lock
		c.UnLock()
		return
	}

	// Remove unwanted properties
	c.UnMaximize()
	c.UnFullscreen()

	// Calculate dimension offsets
	ext := c.Latest.Dimensions.Extents
	dx, dy, dw, dh := 0, 0, 0, 0

	if c.Latest.Dimensions.AdjPos {
		dx, dy = ext.Left, ext.Top
	}
	if c.Latest.Dimensions.AdjSize {
		dw, dh = ext.Left+ext.Right, ext.Top+ext.Bottom
	}

	// Move and/or resize window
	if w > 0 && h > 0 {
		MoveResizeXWindow(c.Window.Id, x+dx, y+dy, w-dw, h-dh)
	} else {
		MoveXWindow(c.Window.Id, x+dx, y+dy)
	}

	// Update stored dimensions
	c.Update()
}

func (c *Client) OuterGeometry() (x, y, w, h int) {

	// Outer window dimensions (x/y relative to workspace)
	oGeom, err := c.Window.Instance.DecorGeometry()
	if err != nil {
		return
	}

	// Inner window dimensions (x/y relative to outer window)
	iGeom, err := xwindow.RawGeometry(X, xproto.Drawable(c.Window.Id))
	if err != nil {
		return
	}

	// Reset inner window positions (some wm won't return x/y relative to outer window)
	if reflect.DeepEqual(oGeom, iGeom) {
		iGeom.XSet(0)
		iGeom.YSet(0)
	}

	// Decoration extents (l/r/t/b relative to outer window dimensions)
	ext := c.Latest.Dimensions.Extents
	dx, dy, dw, dh := ext.Left, ext.Top, ext.Left+ext.Right, ext.Top+ext.Bottom

	// Calculate outer geometry (including server and client decorations)
	x, y, w, h = oGeom.X()+iGeom.X()-dx, oGeom.Y()+iGeom.Y()-dy, iGeom.Width()+dw, iGeom.Height()+dh

	return
}

func (c *Client) Restore(flag uint8) {
	if flag == Latest {
		c.Update()
	} else if flag == Natural {
		c.Update()
	}

	// Restore window sizes
	c.UnLimit()
	c.UnMaximize()
	c.UnFullscreen()

	// Restore window decorations
	if flag == Original {
		if common.Config.WindowDecoration {
			c.Decorate()
		} else {
			c.UnDecorate()
		}
		c.Update()
	}

	// Disable adjustments on restore
	if c.Latest.Dimensions.AdjRestore {
		c.Latest.Dimensions.AdjPos = false
		c.Latest.Dimensions.AdjSize = false
	}

	// Move window to restore position/size.
	geom := c.RestoreGeometry(flag)
	c.MoveWindow(geom.X, geom.Y, geom.Width, geom.Height)
}

func (c *Client) RestoreGeometry(flag uint8) common.Geometry {
	latest := c.Latest.Dimensions.Geometry
	if flag == Original {
		return c.Original.Dimensions.Geometry
	}
	if flag != Natural {
		return latest
	}

	natural := c.Original.Dimensions.Geometry
	if natural.Width <= 0 || natural.Height <= 0 {
		return latest
	}

	center := latest.Center()
	return common.Geometry{
		X:      center.X - natural.Width/2,
		Y:      center.Y - natural.Height/2,
		Width:  natural.Width,
		Height: natural.Height,
	}
}

func (c *Client) Update() {
	info := GetInfo(c.Window.Id)
	if len(info.Class) == 0 {
		return
	}
	log.Debug("Update client info [", info.Class, "]")

	// Update client info
	c.Latest = info
}

func (c *Client) IsNew() bool {
	created := time.UnixMilli(c.Window.Created)
	return time.Since(created) < 1000*time.Millisecond
}

func IsSpecial(info *Info) bool {

	// Check internal windows
	if info.Class == common.Build.Name {
		log.Info("Ignore internal window [", info.Class, "]")
		return true
	}

	// Check window types
	types := []string{
		"_NET_WM_WINDOW_TYPE_DOCK",
		"_NET_WM_WINDOW_TYPE_DESKTOP",
		"_NET_WM_WINDOW_TYPE_TOOLBAR",
		"_NET_WM_WINDOW_TYPE_UTILITY",
		"_NET_WM_WINDOW_TYPE_TOOLTIP",
		"_NET_WM_WINDOW_TYPE_SPLASH",
		"_NET_WM_WINDOW_TYPE_DIALOG",
		"_NET_WM_WINDOW_TYPE_COMBO",
		"_NET_WM_WINDOW_TYPE_NOTIFICATION",
		"_NET_WM_WINDOW_TYPE_DROPDOWN_MENU",
		"_NET_WM_WINDOW_TYPE_POPUP_MENU",
		"_NET_WM_WINDOW_TYPE_MENU",
		"_NET_WM_WINDOW_TYPE_DND",
	}
	for _, typ := range info.Types {
		if common.IsInList(typ, types) {
			log.Info("Ignore window with type ", typ, " [", info.Class, "]")
			return true
		}
	}

	// Check window states
	states := []string{
		"_NET_WM_STATE_HIDDEN",
		"_NET_WM_STATE_MODAL",
		"_NET_WM_STATE_ABOVE",
		"_NET_WM_STATE_BELOW",
		"_NET_WM_STATE_SKIP_PAGER",
		"_NET_WM_STATE_SKIP_TASKBAR",
	}
	for _, state := range info.States {
		if common.IsInList(state, states) {
			log.Info("Ignore window with state ", state, " [", info.Class, "]")
			return true
		}
	}

	return false
}

func IsIgnored(info *Info) bool {

	// Check invalid windows
	if len(info.Class) == 0 {
		log.Info("Ignore invalid window")
		return true
	}

	// Check ignored windows
	for _, s := range common.Config.WindowIgnore {
		if len(s) != 2 {
			log.Warn("Skip malformed window_ignore entry")
			continue
		}
		conf_class := s[0]
		conf_name := s[1]

		reg_class, err := regexp.Compile(strings.ToLower(conf_class))
		if err != nil {
			log.Warn("Skip invalid window_ignore class regex: ", err)
			continue
		}

		// Ignore all windows with this class
		class_match := reg_class.MatchString(strings.ToLower(info.Class))

		// But allow the window with a special name
		name_match := false
		if conf_name != "" {
			reg_name, err := regexp.Compile(strings.ToLower(conf_name))
			if err != nil {
				log.Warn("Skip invalid window_ignore name regex: ", err)
				continue
			}
			name_match = reg_name.MatchString(strings.ToLower(info.Name))
		}

		if class_match && !name_match {
			log.Info("Ignore window with ", strings.TrimSpace(strings.Join(s, " ")), " from config [", info.Class, "]")
			return true
		}
	}

	return false
}

func IsFloating(info *Info) bool {
	if info == nil || len(info.Class) == 0 || info.Class == common.Build.Name {
		return false
	}
	if IsIgnored(info) {
		return true
	}

	floatingTypes := []string{
		"_NET_WM_WINDOW_TYPE_DIALOG",
		"_NET_WM_WINDOW_TYPE_UTILITY",
		"_NET_WM_WINDOW_TYPE_TOOLBAR",
		"_NET_WM_WINDOW_TYPE_SPLASH",
	}
	for _, typ := range info.Types {
		if common.IsInList(typ, floatingTypes) {
			return true
		}
	}

	return false
}

func IsFullscreen(info *Info) bool {
	return common.IsInList("_NET_WM_STATE_FULLSCREEN", info.States)
}

func IsMaximized(info *Info) bool {
	return common.IsInList("_NET_WM_STATE_MAXIMIZED_VERT", info.States) || common.IsInList("_NET_WM_STATE_MAXIMIZED_HORZ", info.States)
}

func IsMinimized(info *Info) bool {
	return common.IsInList("_NET_WM_STATE_HIDDEN", info.States)
}

func IsAbove(info *Info) bool {
	return common.IsInList("_NET_WM_STATE_ABOVE", info.States)
}

func SetAbove(window xproto.Window) bool {
	info := GetInfo(window)
	if IsAbove(info) {
		return false
	}
	return RequestWindowState(window, ewmh.StateAdd, "_NET_WM_STATE_ABOVE")
}

func UnsetAbove(window xproto.Window) bool {
	info := GetInfo(window)
	if !IsAbove(info) {
		return false
	}
	return RequestWindowState(window, ewmh.StateRemove, "_NET_WM_STATE_ABOVE")
}

func IsSticky(info *Info) bool {
	return common.IsInList("_NET_WM_STATE_STICKY", info.States)
}

func SetSticky(window xproto.Window) bool {
	info := GetInfo(window)
	changed := false

	if !IsSticky(info) {
		if !RequestWindowState(window, ewmh.StateAdd, "_NET_WM_STATE_STICKY") {
			return false
		}
		changed = true
	}

	if !IsAbove(info) {
		if !RequestWindowState(window, ewmh.StateAdd, "_NET_WM_STATE_ABOVE") {
			return changed
		}
		changed = true
	}

	desktop := ^uint32(0)
	if !SetWindowDesktop(window, desktop) {
		return changed
	}
	return changed
}

func GetInfo(w xproto.Window) *Info {
	var err error

	var class string
	var name string
	var types []string
	var states []string
	var location Location
	var dimensions Dimensions

	// Window class (internal class name of the window)
	cls, err := icccm.WmClassGet(X, w)
	if err != nil {
		log.Trace("Error on request: ", err)
	} else if cls != nil {
		class = cls.Class
	}

	// Window name (title on top of the window)
	name, err = icccm.WmNameGet(X, w)
	if err != nil {
		name = class
	}

	// Window geometry (dimensions of the window)
	geom, err := CreateXWindow(w).Instance.DecorGeometry()
	if err != nil {
		geom = &xrect.XRect{}
	}

	// Window desktop and screen (window workspace location)
	desktop, err := ewmh.WmDesktopGet(X, w)
	sticky := desktop > Workplace.DesktopCount
	if err != nil || sticky {
		desktop = CurrentDesktopGet(X)
	}
	location = Location{
		Desktop: desktop,
		Screen:  ScreenGet(common.CreateGeometry(geom).Center()),
	}

	// Window types (types of the window)
	types, err = ewmh.WmWindowTypeGet(X, w)
	if err != nil {
		types = []string{}
	}

	// Window states (states of the window)
	states, err = ewmh.WmStateGet(X, w)
	if err != nil {
		states = []string{}
	}
	if sticky && !common.IsInList("_NET_WM_STATE_STICKY", states) {
		states = append(states, "_NET_WM_STATE_STICKY")
	}

	// Window normal hints (normal hints of the window)
	nhints, err := icccm.WmNormalHintsGet(X, w)
	if err != nil {
		nhints = &icccm.NormalHints{}
	}

	// Window motif hints (hints of the window)
	mhints, err := motif.WmHintsGet(X, w)
	if err != nil {
		mhints = &motif.Hints{}
	}

	// Window extents (server/client decorations of the window)
	extNet, _ := xprop.PropValNums(xprop.GetProperty(X, w, "_NET_FRAME_EXTENTS"))
	extGtk, _ := xprop.PropValNums(xprop.GetProperty(X, w, "_GTK_FRAME_EXTENTS"))

	ext := FrameExtents(extNet, extGtk)

	// Window dimensions (geometry/extent information for move/resize)
	dimensions = Dimensions{
		Geometry: *common.CreateGeometry(geom),
		Hints: Hints{
			Normal: *nhints,
			Motif:  *mhints,
		},
		Extents: ewmh.FrameExtents{
			Left:   ext.Left,
			Right:  ext.Right,
			Top:    ext.Top,
			Bottom: ext.Bottom,
		},
		AdjPos:     (nhints.WinGravity > 1 && !common.AllZero(extNet)) || !common.AllZero(extGtk),
		AdjSize:    !common.AllZero(extNet) || !common.AllZero(extGtk),
		AdjRestore: !common.AllZero(extGtk),
	}

	return &Info{
		Class:      class,
		Name:       name,
		Types:      types,
		States:     states,
		Location:   location,
		Dimensions: dimensions,
	}
}

func FrameExtents(net, gtk []uint) ewmh.FrameExtents {
	values := make([]int, 4)
	for i, e := range net {
		if i >= len(values) {
			break
		}
		values[i] += int(e)
	}
	for i, e := range gtk {
		if i >= len(values) {
			break
		}
		values[i] -= int(e)
		if values[i] < 0 {
			values[i] = 0
		}
	}

	return ewmh.FrameExtents{
		Left:   values[0],
		Right:  values[1],
		Top:    values[2],
		Bottom: values[3],
	}
}
