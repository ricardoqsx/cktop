package tui

type LayoutMode int

const (
	LayoutWide LayoutMode = iota
	LayoutMedium
	LayoutCompact
	LayoutMinimal
)

type Layout struct {
	Mode         LayoutMode
	Width        int
	Height       int
	ContentWidth int
	Framed       bool
}

func ResolveLayout(width, height int) Layout {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	mode := LayoutWide
	switch {
	case width < 50 || height < 12:
		mode = LayoutMinimal
	case width < 80 || height < 20:
		mode = LayoutCompact
	case width < 120:
		mode = LayoutMedium
	}

	framed := width >= 80 && height >= 16
	contentWidth := width
	if framed {
		// Border and horizontal padding consume six cells.
		contentWidth = width - 6
	}
	if contentWidth < 1 {
		contentWidth = 1
	}

	return Layout{
		Mode:         mode,
		Width:        width,
		Height:       height,
		ContentWidth: contentWidth,
		Framed:       framed,
	}
}
