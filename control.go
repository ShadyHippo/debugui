// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package debugui

import (
	"fmt"
	"image"
	"iter"
	"math"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type controlID string

const emptyControlID controlID = ""

type option int

const (
	optionAlignCenter option = (1 << iota)
	optionAlignRight
	optionNoInteract
	optionNoFrame
	optionNoResize
	optionNoScroll
	optionNoClose
	optionNoTitle
	optionHoldFocus
	optionAutoSize
	optionPopup
	optionClosed
	optionExpanded
)

func (c *Context) inHoverRoot() bool {
	for i := len(c.containerStack) - 1; i >= 0; i-- {
		if c.containerStack[i] == c.hoverRoot {
			return true
		}
		// only root containers have their `head` field set; stop searching if we've
		// reached the current root container
		if c.containerStack[i].headIdx >= 0 {
			break
		}
	}
	return false
}

func (c *Context) mouseOver(bounds image.Rectangle) bool {
	p := c.cursorPosition()
	return p.In(bounds) && p.In(c.clipRect()) && c.inHoverRoot()
}

func (c *Context) mouseDelta() image.Point {
	return c.cursorPosition().Sub(c.lastMousePos)
}

func (c *Context) cursorPosition() image.Point {
	p := image.Pt(ebiten.CursorPosition())
	p.X /= c.Scale()
	p.Y /= c.Scale()
	return p
}

func (c *Context) updateControl(id controlID, bounds image.Rectangle, opt option) (wasFocused bool) {
	if id == emptyControlID {
		return false
	}

	mouseover := c.mouseOver(bounds)

	if c.focus == id {
		c.keepFocus = true
	}
	if (opt & optionNoInteract) != 0 {
		return false
	}
	if mouseover && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		c.hover = id
	}

	if c.focus == id {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !mouseover {
			c.setFocus(emptyControlID)
			wasFocused = true
		}
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && (^opt&optionHoldFocus) != 0 {
			c.setFocus(emptyControlID)
			wasFocused = true
		}
	}

	if c.hover == id {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			c.setFocus(id)
		} else if !mouseover {
			c.hover = emptyControlID
		}
	}

	return
}

func (c *Context) Control(f func(bounds image.Rectangle) bool) bool {
	pc := caller()
	id := c.idFromCaller(pc)
	var res bool
	c.wrapError(func() error {
		var err error
		res, err = c.control(id, 0, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
			return f(bounds), nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	return res
}

func (c *Context) control(id controlID, opt option, f func(bounds image.Rectangle, wasFocused bool) (bool, error)) (bool, error) {
	r, err := c.layoutNext()
	if err != nil {
		return false, err
	}
	wasFocused := c.updateControl(id, r, opt)
	res, err := f(r, wasFocused)
	if err != nil {
		return false, err
	}
	return res, nil
}

func removeSpaceAtLineTail(str string) string {
	return strings.TrimRightFunc(str, unicode.IsSpace)
}

func lines(text string, width int) iter.Seq[string] {
	return func(yield func(string) bool) {
		var line string
		var word string
		state := -1
		for len(text) > 0 {
			cluster, nextText, boundaries, nextState := uniseg.StepString(text, state)
			switch m := boundaries & uniseg.MaskLine; m {
			default:
				word += cluster
			case uniseg.LineCanBreak, uniseg.LineMustBreak:
				if line == "" {
					line += word + cluster
				} else {
					l := removeSpaceAtLineTail(line + word + cluster)
					if textWidth(l) > width {
						if !yield(removeSpaceAtLineTail(line)) {
							return
						}
						line = word + cluster
					} else {
						line += word + cluster
					}
				}
				word = ""
				if m == uniseg.LineMustBreak {
					if !yield(removeSpaceAtLineTail(line)) {
						return
					}
					line = ""
				}
			}
			state = nextState
			text = nextText
		}

		line += word
		if len(line) > 0 {
			if !yield(removeSpaceAtLineTail(line)) {
				return
			}
		}
	}
}

// Text creates a text label.
func (c *Context) Text(text string) {
	c.wrapError(func() error {
		if err := c.gridCell(func(bounds image.Rectangle) error {
			c.SetGridLayout([]int{-1}, []int{lineHeight()})
			for line := range lines(text, bounds.Dx()-c.style().padding) {
				if _, err := c.control(emptyControlID, 0, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
					c.drawControlText(line, bounds, colorText, 0)
					return false, nil
				}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
}

func (c *Context) button(label string, opt option, callerPC uintptr) (controlID, bool, error) {
	id := c.idFromCaller(callerPC)
	res, err := c.control(id, opt, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
		var res bool
		// handle click
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && c.focus == id {
			res = true
		}
		// draw
		c.drawControlFrame(id, bounds, colorButton, opt)
		if len(label) > 0 {
			c.drawControlText(label, bounds, colorText, opt)
		}
		return res, nil
	})
	if err != nil {
		return emptyControlID, false, err
	}
	return id, res, nil
}

// Checkbox creates a checkbox with the given boolean state and text label.
//
// A Checkbox control is uniquely determined by its call location.
// Function calls made in different locations will create different controls.
// If you want to generate different controls with the same function call in a loop (such as a for loop), use [IDScope].
func (c *Context) Checkbox(state *bool, label string) bool {
	pc := caller()
	id := c.idFromCaller(pc)
	var res bool
	c.wrapError(func() error {
		var err error
		res, err = c.control(id, 0, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
			var res bool
			box := image.Rect(bounds.Min.X, bounds.Min.Y+(bounds.Dy()-lineHeight())/2, bounds.Min.X+lineHeight(), bounds.Max.Y-(bounds.Dy()-lineHeight())/2)
			c.updateControl(id, bounds, 0)
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && c.focus == id {
				res = true
				*state = !*state
			}
			c.drawControlFrame(id, box, colorBase, 0)
			if *state {
				c.drawIcon(iconCheck, box, c.style().colors[colorText])
			}
			if label != "" {
				bounds = image.Rect(bounds.Min.X+lineHeight(), bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
				c.drawControlText(label, bounds, colorText, 0)
			}
			return res, nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	return res
}

func (c *Context) slider(value *int, low, high, step int, id controlID, opt option) (bool, error) {
	last := *value
	v := last

	res, err := c.numberTextField(&v, id)
	if err != nil {
		return false, err
	}
	if res {
		*value = v
		return false, nil
	}

	res, err = c.control(id, opt, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
		var res bool
		if c.focus == id && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if w := bounds.Dx() - defaultStyle.thumbSize; w > 0 {
				v = low + (c.cursorPosition().X-bounds.Min.X)*(high-low)/w
			}
			if step != 0 {
				v = v / step * step
			}
		}
		*value = clamp(v, low, high)
		v = *value
		if last != v {
			res = true
		}

		c.drawControlFrame(id, bounds, colorBase, opt)
		w := c.style().thumbSize
		x := int((v - low) * (bounds.Dx() - w) / (high - low))
		thumb := image.Rect(bounds.Min.X+x, bounds.Min.Y, bounds.Min.X+x+w, bounds.Max.Y)
		c.drawControlFrame(id, thumb, colorButton, opt)
		text := fmt.Sprintf("%d", v)
		c.drawControlText(text, bounds, colorText, opt)

		return res, nil
	})
	if err != nil {
		return false, err
	}
	return res, nil
}

func (c *Context) sliderF(value *float64, low, high, step float64, digits int, id controlID, opt option) (bool, error) {
	last := *value
	v := last

	res, err := c.numberTextFieldF(&v, id)
	if err != nil {
		return false, err
	}
	if res {
		*value = v
		return false, nil
	}

	res, err = c.control(id, opt, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
		var res bool
		if c.focus == id && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if w := float64(bounds.Dx() - defaultStyle.thumbSize); w > 0 {
				v = low + float64(c.cursorPosition().X-bounds.Min.X)*(high-low)/w
			}
			if step != 0 {
				v = math.Round(v/step) * step
			}
		}
		*value = clamp(v, low, high)
		v = *value
		if last != v {
			res = true
		}

		c.drawControlFrame(id, bounds, colorBase, opt)
		w := c.style().thumbSize
		x := int((v - low) * float64(bounds.Dx()-w) / (high - low))
		thumb := image.Rect(bounds.Min.X+x, bounds.Min.Y, bounds.Min.X+x+w, bounds.Max.Y)
		c.drawControlFrame(id, thumb, colorButton, opt)
		text := formatNumber(v, digits)
		c.drawControlText(text, bounds, colorText, opt)

		return res, nil
	})
	if err != nil {
		return false, err
	}
	return res, nil
}

func (c *Context) header(label string, isTreeNode bool, opt option, callerPC uintptr, f func() error) error {
	id := c.idFromCaller(callerPC)
	c.SetGridLayout(nil, nil)

	var expanded bool
	toggled := c.currentContainer().toggled(id)
	if (opt & optionExpanded) != 0 {
		expanded = !toggled
	} else {
		expanded = toggled
	}

	res, err := c.control(id, 0, func(bounds image.Rectangle, wasFocused bool) (bool, error) {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && c.focus == id {
			c.currentContainer().toggle(id)
		}
		if isTreeNode {
			if c.hover == id {
				c.drawFrame(bounds, colorButtonHover)
			}
		} else {
			c.drawControlFrame(id, bounds, colorButton, 0)
		}
		var icon icon
		if expanded {
			icon = iconExpanded
		} else {
			icon = iconCollapsed
		}
		c.drawIcon(
			icon,
			image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+bounds.Dy(), bounds.Max.Y),
			c.style().colors[colorText],
		)
		bounds.Min.X += bounds.Dy() - c.style().padding
		c.drawControlText(label, bounds, colorText, 0)

		return expanded, nil
	})
	if err != nil {
		return err
	}
	if res {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) treeNode(label string, opt option, callerPC uintptr, f func()) error {
	if err := c.header(label, true, opt, callerPC, func() (err error) {
		l, err := c.layout()
		if err != nil {
			return err
		}
		l.indent += c.style().indent
		defer func() {
			l, err2 := c.layout()
			if err2 != nil && err == nil {
				err = err2
				return
			}
			l.indent -= c.style().indent
		}()
		f()
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// x = x, y = y, w = w, h = h
func (c *Context) scrollbarVertical(cnt *container, b image.Rectangle, cs image.Point) {
	maxscroll := cs.Y - b.Dy()
	if maxscroll > 0 && b.Dy() > 0 {
		// get sizing / positioning
		base := b
		base.Min.X = b.Max.X
		base.Max.X = base.Min.X + c.style().scrollbarSize

		// handle input
		id := c.idFromString("scrollbar-y")
		c.updateControl(id, base, 0)
		if c.focus == id && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			cnt.layout.ScrollOffset.Y += c.mouseDelta().Y * cs.Y / base.Dy()
		}
		// clamp scroll to limits
		cnt.layout.ScrollOffset.Y = clamp(cnt.layout.ScrollOffset.Y, 0, maxscroll)

		// draw base and thumb
		c.drawFrame(base, colorScrollBase)
		thumb := base
		thumb.Max.Y = thumb.Min.Y + max(c.style().thumbSize, base.Dy()*b.Dy()/cs.Y)
		thumb = thumb.Add(image.Pt(0, cnt.layout.ScrollOffset.Y*(base.Dy()-thumb.Dy())/maxscroll))
		c.drawFrame(thumb, colorScrollThumb)

		// set this as the scroll_target (will get scrolled on mousewheel)
		// if the mouse is over it
		if c.mouseOver(b) {
			c.scrollTarget = cnt
		}
	} else {
		cnt.layout.ScrollOffset.Y = 0
	}
}

// x = y, y = x, w = h, h = w
func (c *Context) scrollbarHorizontal(cnt *container, b image.Rectangle, cs image.Point) {
	maxscroll := cs.X - b.Dx()
	if maxscroll > 0 && b.Dx() > 0 {
		// get sizing / positioning
		base := b
		base.Min.Y = b.Max.Y
		base.Max.Y = base.Min.Y + c.style().scrollbarSize

		// handle input
		id := c.idFromString("scrollbar-x")
		c.updateControl(id, base, 0)
		if c.focus == id && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			cnt.layout.ScrollOffset.X += c.mouseDelta().X * cs.X / base.Dx()
		}
		// clamp scroll to limits
		cnt.layout.ScrollOffset.X = clamp(cnt.layout.ScrollOffset.X, 0, maxscroll)

		// draw base and thumb
		c.drawFrame(base, colorScrollBase)
		thumb := base
		thumb.Max.X = thumb.Min.X + max(c.style().thumbSize, base.Dx()*b.Dx()/cs.X)
		thumb = thumb.Add(image.Pt(cnt.layout.ScrollOffset.X*(base.Dx()-thumb.Dx())/maxscroll, 0))
		c.drawFrame(thumb, colorScrollThumb)

		// set this as the scroll_target (will get scrolled on mousewheel)
		// if the mouse is over it
		if c.mouseOver(b) {
			c.scrollTarget = cnt
		}
	} else {
		cnt.layout.ScrollOffset.X = 0
	}
}

// if `swap` is true, X = Y, Y = X, W = H, H = W
func (c *Context) scrollbar(cnt *container, b image.Rectangle, cs image.Point, swap bool) {
	if swap {
		c.scrollbarHorizontal(cnt, b, cs)
	} else {
		c.scrollbarVertical(cnt, b, cs)
	}
}

func (c *Context) scrollbars(cnt *container, body image.Rectangle) image.Rectangle {
	sz := c.style().scrollbarSize
	cs := cnt.layout.ContentSize
	cs.X += c.style().padding * 2
	cs.Y += c.style().padding * 2
	c.pushClipRect(body)
	// resize body to make room for scrollbars
	if cs.Y > cnt.layout.BodyBounds.Dy() {
		body.Max.X -= sz
	}
	if cs.X > cnt.layout.BodyBounds.Dx() {
		body.Max.Y -= sz
	}
	// to create a horizontal or vertical scrollbar almost-identical code is
	// used; only the references to `x|y` `w|h` need to be switched
	c.scrollbar(cnt, body, cs, false)
	c.scrollbar(cnt, body, cs, true)
	c.popClipRect()
	return body
}

func (c *Context) pushContainerBodyLayout(cnt *container, body image.Rectangle, opt option) error {
	if (^opt & optionNoScroll) != 0 {
		body = c.scrollbars(cnt, body)
	}
	if err := c.pushLayout(body.Inset(c.style().padding), cnt.layout.ScrollOffset, opt&optionAutoSize != 0); err != nil {
		return err
	}
	cnt.layout.BodyBounds = body
	return nil
}

// SetScale sets the scale of the UI.
//
// The scale affects the rendering result of the UI.
//
// The default scale is 1.
func (c *Context) SetScale(scale int) {
	if scale < 1 {
		panic("debugui: scale must be >= 1")
	}
	c.scaleMinus1 = scale - 1
}

// Scale returns the scale of the UI.
func (c *Context) Scale() int {
	return c.scaleMinus1 + 1
}

func (c *Context) style() *style {
	return &defaultStyle
}

func (c *Context) isCapturingInput() bool {
	if c.err != nil {
		return false
	}

	return c.hoverRoot != nil || c.focus != emptyControlID
}
