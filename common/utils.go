package common

import (
	"math"
	"strconv"
	"strings"

	"crypto/sha1"
	"encoding/hex"

	"github.com/jezek/xgbutil/xrect"
)

type Point struct {
	X int // Object point x position
	Y int // Object point y position
}

func CreatePoint(x int, y int) *Point {
	return &Point{
		X: x,
		Y: y,
	}
}

type Geometry struct {
	X      int // Object geometry x position
	Y      int // Object geometry y position
	Width  int // Object geometry width dimension
	Height int // Object geometry height dimension
}

func CreateGeometry(r xrect.Rect) *Geometry {
	return &Geometry{
		X:      r.X(),
		Y:      r.Y(),
		Width:  r.Width(),
		Height: r.Height(),
	}
}

func (g *Geometry) Center() Point {
	return *CreatePoint(g.X+g.Width/2, g.Y+g.Height/2)
}

func (g *Geometry) Rect() xrect.Rect {
	return xrect.New(g.X, g.Y, g.Width, g.Height)
}

func (g *Geometry) Pieces() (int, int, int, int) {
	return g.X, g.Y, g.Width, g.Height
}

type Map = map[string]interface{} // Generic map type
type List = []Map                 // Generic list type

func HashString(text string, max int) string {
	hash := sha1.New()
	hash.Write([]byte(text))
	str := hex.EncodeToString(hash.Sum(nil))
	return TruncateString(str, max)
}

func TruncateString(s string, max int) string {
	if max > len(s) {
		return s
	}
	return s[:max]
}

func RemoveChars(s string, chars []string) string {
	for _, c := range chars {
		s = strings.Replace(s, c, "", -1)
	}
	return s
}

func AllZero(items []uint) bool {
	mask := uint(0)
	for _, item := range items {
		mask |= item
	}
	return mask == 0
}

func AllTrue(items []bool) bool {
	mask := true
	for _, item := range items {
		mask = mask && item
	}
	return mask
}

func IsInsideRect(p Point, g Geometry) bool {
	x, y, w, h := g.Pieces()
	return p.X >= x && p.X <= (x+w) && p.Y >= y && p.Y <= (y+h)
}

func IsInList(item string, items []string) bool {
	for i := 0; i < len(items); i++ {
		if items[i] == item {
			return true
		}
	}
	return false
}

func IsInMap(m Map, keys []string) bool {
	exists := true
	for _, key := range keys {
		_, exist := m[key]
		exists = exists && exist
	}
	return exists
}

func ReverseList[T any](items []T) []T {
	for i, j := 0, len(items)-1; i < j; {
		items[i], items[j] = items[j], items[i]
		i++
		j--
	}
	return items
}

func StringsToInts(items []string) []int {
	result := make([]int, len(items))
	for i, item := range items {
		integer, err := strconv.Atoi(item)
		if err != nil {
			integer = -1
		}
		result[i] = integer
	}
	return result
}

func MinInt(a int, b int) int {
	return int(math.Min(float64(a), float64(b)))
}

func MaxInt(a int, b int) int {
	return int(math.Max(float64(a), float64(b)))
}

type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

type DirectionScore struct {
	Distance  int
	Primary   int
	Secondary int
}

func ScoreDirection(source Geometry, target Geometry, direction Direction) (DirectionScore, bool) {
	sourceCenter := source.Center()
	targetCenter := target.Center()
	dx, dy := targetCenter.X-sourceCenter.X, targetCenter.Y-sourceCenter.Y
	score := DirectionScore{}

	switch direction {
	case Up:
		if dy >= 0 {
			return score, false
		}
		score.Primary = MaxInt(source.Y-(target.Y+target.Height), 0)
		score.Secondary = intervalGap(source.X, source.X+source.Width, target.X, target.X+target.Width)
	case Down:
		if dy <= 0 {
			return score, false
		}
		score.Primary = MaxInt(target.Y-(source.Y+source.Height), 0)
		score.Secondary = intervalGap(source.X, source.X+source.Width, target.X, target.X+target.Width)
	case Left:
		if dx >= 0 {
			return score, false
		}
		score.Primary = MaxInt(source.X-(target.X+target.Width), 0)
		score.Secondary = intervalGap(source.Y, source.Y+source.Height, target.Y, target.Y+target.Height)
	case Right:
		if dx <= 0 {
			return score, false
		}
		score.Primary = MaxInt(target.X-(source.X+source.Width), 0)
		score.Secondary = intervalGap(source.Y, source.Y+source.Height, target.Y, target.Y+target.Height)
	default:
		return score, false
	}

	score.Distance = score.Primary + score.Secondary
	// Center alignment provides deterministic ordering when rectangle-edge
	// distances are otherwise identical.
	if direction == Up || direction == Down {
		score.Secondary = AbsInt(dx)
	} else {
		score.Secondary = AbsInt(dy)
	}

	return score, true
}

func BetterDirectionScore(candidate DirectionScore, current DirectionScore) bool {
	if candidate.Distance != current.Distance {
		return candidate.Distance < current.Distance
	}
	if candidate.Primary != current.Primary {
		return candidate.Primary < current.Primary
	}
	return candidate.Secondary < current.Secondary
}

func intervalGap(aStart int, aEnd int, bStart int, bEnd int) int {
	if aEnd < bStart {
		return bStart - aEnd
	}
	if bEnd < aStart {
		return aStart - bEnd
	}
	return 0
}

func AbsInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
