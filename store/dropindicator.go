package store

import (
	"image"

	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgbutil/ewmh"
	"github.com/jezek/xgbutil/icccm"
	"github.com/jezek/xgbutil/motif"
	"github.com/jezek/xgbutil/xgraphics"
	"github.com/jezek/xgbutil/xwindow"

	"github.com/billmakes/bsptile/v2/common"

	log "github.com/sirupsen/logrus"
)

var (
	indicatorWindows [5]*xwindow.Window // 0-3: outline edges (top/bottom/left/right). 4: optional fill.
	indicatorTarget  xproto.Window
)

type DropZone struct {
	Target xproto.Window
	Edge   string
	X, Y   int
	W, H   int
}

func ShowDropIndicator(z *DropZone) {
	if z == nil {
		HideDropIndicator()
		return
	}
	key := xproto.Window(uint32(z.Target) ^ atomFromEdge(z.Edge))
	if key == indicatorTarget && indicatorWindows[0] != nil {
		return
	}
	HideDropIndicator()

	x, y, w, h := z.X, z.Y, z.W, z.H
	if w <= 0 || h <= 0 {
		return
	}
	t := common.Config.DropTargetWidth
	if t <= 0 {
		t = 4
	}

	outline := bgra("gui_drop_target")
	if (outline == xgraphics.BGRA{}) {
		outline = bgra("gui_client_master")
	}
	outlineColor := outline
	outlineColor.A = 0xFF

	// Optional fill window inside the outline rect.
	fillRaw := common.Config.Colors["gui_drop_target_fill"]
	if len(fillRaw) == 4 && fillRaw[3] > 0 {
		fillColor := xgraphics.BGRA{R: uint8(fillRaw[0]), G: uint8(fillRaw[1]), B: uint8(fillRaw[2]), A: 0xFF}
		opacity := float64(fillRaw[3]) / 255.0
		fx, fy, fw, fh := x+t, y+t, w-2*t, h-2*t
		if fw > 0 && fh > 0 {
			indicatorWindows[4] = createIndicatorWindow(fx, fy, fw, fh, fillColor, opacity)
		}
	}

	rects := [4][4]int{
		{x, y, w, t},                   // top
		{x, y + h - t, w, t},           // bottom
		{x, y + t, t, h - 2*t},         // left
		{x + w - t, y + t, t, h - 2*t}, // right
	}
	for i, r := range rects {
		if r[2] <= 0 || r[3] <= 0 {
			continue
		}
		indicatorWindows[i] = createIndicatorWindow(r[0], r[1], r[2], r[3], outlineColor, 1.0)
	}
	indicatorTarget = key
}

func HideDropIndicator() {
	for i, w := range indicatorWindows {
		if w != nil {
			w.Destroy()
			indicatorWindows[i] = nil
		}
	}
	indicatorTarget = 0
}

func atomFromEdge(edge string) uint32 {
	switch edge {
	case "left":
		return 0x10000
	case "right":
		return 0x20000
	case "top":
		return 0x30000
	case "bottom":
		return 0x40000
	}
	return 0
}

func createIndicatorWindow(x, y, w, h int, color xgraphics.BGRA, opacity float64) *xwindow.Window {
	win, err := xwindow.Generate(X)
	if err != nil {
		log.Error("Drop indicator window create failed: ", err)
		return nil
	}

	if err := win.CreateChecked(X.RootWin(), x, y, w, h, xproto.CwOverrideRedirect, 1); err != nil {
		log.Error("Drop indicator window create failed: ", err)
		return nil
	}

	icccm.WmClassSet(win.X, win.Id, &icccm.WmClass{
		Instance: common.Build.Name,
		Class:    common.Build.Name,
	})
	ewmh.WmStateSet(win.X, win.Id, []string{
		"_NET_WM_STATE_SKIP_TASKBAR",
		"_NET_WM_STATE_SKIP_PAGER",
		"_NET_WM_STATE_ABOVE",
	})
	motif.WmHintsSet(win.X, win.Id, &motif.Hints{
		Flags:      motif.HintFunctions | motif.HintDecorations,
		Function:   motif.FunctionNone,
		Decoration: motif.DecorationNone,
	})

	if opacity < 1.0 {
		if err := ewmh.WmWindowOpacitySet(win.X, win.Id, opacity); err != nil {
			log.Warn("Drop indicator opacity set failed: ", err)
		}
	}

	img := xgraphics.New(win.X, image.Rect(0, 0, w, h))
	img.For(func(_, _ int) xgraphics.BGRA { return color })
	if err := img.XSurfaceSet(win.Id); err != nil {
		log.Warn("Drop indicator surface set failed: ", err)
		win.Destroy()
		return nil
	}
	img.XDraw()
	img.XPaint(win.Id)
	win.Map()
	return win
}

func bgra(name string) xgraphics.BGRA {
	rgba := common.Config.Colors[name]
	if len(rgba) != 4 {
		return xgraphics.BGRA{}
	}
	return xgraphics.BGRA{
		R: uint8(rgba[0]),
		G: uint8(rgba[1]),
		B: uint8(rgba[2]),
		A: uint8(rgba[3]),
	}
}
