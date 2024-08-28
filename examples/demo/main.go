// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package main

import (
	"bytes"
	"image"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/ebitengine/microui"
)

var (
	src  *text.GoTextFaceSource
	face text.Face
)

func init() {
	var err error

	src, err = text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		log.Fatal("err: ", err)
	}

	face = &text.GoTextFace{
		Source: src,
		Size:   14,
	}
}

type Game struct {
	ctx *microui.Context

	cx, cy   int
	commands []*microui.Command
}

func New() *Game {
	ctx := microui.NewContext()
	ctx.TextWidth = func(font microui.Font, str string) int {
		return int(text.Advance(str, face))
	}
	ctx.TextHeight = func(font microui.Font) int {
		return 14
	}
	/*ctx.Style = &microui.Style{
		Font:          nil,
		Size:          microui.Vec2{},
		Padding:       0,
		Spacing:       0,
		Indent:        0,
		TitleHeight:   0,
		ScrollbarSize: 0,
		ThumbSize:     0,
		Colors:        [14]microui.Color{
			microui.MU_COLOR_BUTTONHOVER: microui.NewColor(255,0,0,255),
		},
	}*/

	return &Game{
		ctx: ctx,
	}
}

func (g *Game) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	// Inputs
	cx, cy := ebiten.CursorPosition()
	if cx != g.cx || cy != g.cy {
		g.ctx.InputMouseMove(cx, cy)
		g.cx, g.cy = cx, cy
	}
	wx, wy := ebiten.Wheel()
	if wx != 0 || wy != 0 {
		g.ctx.InputScroll(int(wx*-30), int(wy*-30))
	}
	chars := ebiten.AppendInputChars(nil)
	if len(chars) > 0 {
		g.ctx.InputText(chars)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.ctx.InputMouseDown(cx, cy, ebiten.MouseButtonLeft)
	} else if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		g.ctx.InputMouseUp(cx, cy, ebiten.MouseButtonLeft)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.ctx.InputMouseDown(cx, cy, ebiten.MouseButtonRight)
	} else if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) {
		g.ctx.InputMouseUp(cx, cy, ebiten.MouseButtonRight)
	}
	for _, k := range []ebiten.Key{ebiten.KeyAlt, ebiten.KeyBackspace, ebiten.KeyControl, ebiten.KeyEnter, ebiten.KeyShift} {
		if inpututil.IsKeyJustPressed(k) {
			g.ctx.InputKeyDown(k)
		} else if inpututil.IsKeyJustReleased(k) {
			g.ctx.InputKeyUp(k)
		}
	}

	// UI
	ProcessFrame(g.ctx)

	g.commands = g.commands[:0]
	var cmd *microui.Command
	for g.ctx.NextCommand(&cmd) {
		g.commands = append(g.commands, cmd)
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	target := screen
	for _, cmd := range g.commands {
		switch cmd.Type {
		case microui.CommandRect:
			vector.DrawFilledRect(
				target,
				float32(cmd.Rect.Rect.X),
				float32(cmd.Rect.Rect.Y),
				float32(cmd.Rect.Rect.W),
				float32(cmd.Rect.Rect.H),
				cmd.Rect.Color,
				false,
			)
		case microui.CommandText:
			geom := ebiten.GeoM{}
			geom.Translate(
				float64(cmd.Text.Pos.X),
				float64(cmd.Text.Pos.Y),
			)
			cs := ebiten.ColorScale{}
			cs.ScaleWithColor(cmd.Text.Color)
			text.Draw(target, cmd.Text.Str, face, &text.DrawOptions{
				DrawImageOptions: ebiten.DrawImageOptions{
					GeoM:       geom,
					ColorScale: cs,
				},
			})
		case microui.CommandIcon:
			vector.DrawFilledRect(
				target,
				float32(cmd.Icon.Rect.X),
				float32(cmd.Icon.Rect.Y),
				float32(cmd.Icon.Rect.W),
				float32(cmd.Icon.Rect.H),
				cmd.Icon.Color,
				false,
			)
		case microui.CommandClip:
			target = screen.SubImage(image.Rect(
				cmd.Clip.Rect.X,
				cmd.Clip.Rect.Y,
				min(cmd.Clip.Rect.X+cmd.Clip.Rect.W, screen.Bounds().Dx()),
				min(cmd.Clip.Rect.Y+cmd.Clip.Rect.H, screen.Bounds().Dy()),
			)).(*ebiten.Image)
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 1280, 960
}

func main() {
	ebiten.SetWindowTitle("Ebitengine Microui Demo")
	ebiten.SetWindowSize(1280, 960)
	if err := ebiten.RunGame(New()); err != nil {
		log.Fatal("err: ", err)
	}
}
