// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package debugui

import (
	"image"
	"strings"
)

const idSeparator = "\x00"

func (c *Context) Button(label string) Response {
	label, idStr, _ := strings.Cut(label, idSeparator)
	return c.button(label, idStr, optionAlignCenter)
}

func (c *Context) TextBox(buf *string) Response {
	return c.textBox(buf, 0)
}

func (c *Context) Slider(value *float64, lo, hi float64, step float64, digits int) Response {
	return c.slider(value, lo, hi, step, digits, optionAlignCenter)
}

func (c *Context) Number(value *float64, step float64, digits int) Response {
	return c.number(value, step, digits, optionAlignCenter)
}

func (c *Context) Header(label string, expanded bool) Response {
	var opt option
	if expanded {
		opt |= optionExpanded
	}
	label, idStr, _ := strings.Cut(label, idSeparator)
	return c.header(label, idStr, false, opt)
}

func (c *Context) TreeNode(label string, f func(res Response)) {
	label, idStr, _ := strings.Cut(label, idSeparator)
	c.treeNode(label, idStr, 0, f)
}

func (c *Context) Window(title string, rect image.Rectangle, f func(res Response, layout Layout)) {
	title, idStr, _ := strings.Cut(title, idSeparator)
	c.window(title, idStr, rect, 0, f)
}

func (c *Context) Panel(name string, f func(layout Layout)) {
	c.panel(name, 0, f)
}
