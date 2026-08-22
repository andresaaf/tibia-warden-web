// Package mapimg renders a small static PNG crop of the Tibia world map centred
// on a coordinate, with a marker at the centre. It is used for the Discord embed
// image and the website announcement-card preview. Tiles come from the public,
// MIT-licensed tibia-map-data project on GitHub Pages (256x256 tiles named by
// their absolute top-left world coordinate and floor); 1 in-game coordinate maps
// to exactly 1 pixel, so the coordinate math is a plain linear transform.
package mapimg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"sync"
	"time"
)

const (
	tileSize = 256

	// World bounds of the current map (from the tibia-map viewer). Coordinates
	// outside this are treated as invalid; tiles that simply do not exist inside
	// it render as dark background.
	WorldMinX = 31744
	WorldMaxX = 34559
	WorldMinY = 30976
	WorldMaxY = 33279
	MinZ      = 0
	MaxZ      = 15

	// Output window size, in world coordinates == pixels.
	windowW = 480
	windowH = 320

	tileBaseURL = "https://tibiamaps.github.io/tibia-map-data/mapper"
)

// ValidCoord reports whether (x, y, z) is within the known Tibia map bounds.
func ValidCoord(x, y, z int) bool {
	return x >= WorldMinX && x <= WorldMaxX &&
		y >= WorldMinY && y <= WorldMaxY &&
		z >= MinZ && z <= MaxZ
}

var httpClient = &http.Client{Timeout: 6 * time.Second}

// cache holds rendered PNGs keyed by "x_y_z". A coordinate is immutable per
// announcement so entries never go stale; growth is bounded by cacheMax.
var (
	cacheMu  sync.Mutex
	cache    = map[string][]byte{}
	cacheMax = 512
)

func cacheKey(x, y, z int) string { return fmt.Sprintf("%d_%d_%d", x, y, z) }

// Render returns a PNG map crop centred on (x, y) at floor z, with a marker at
// the centre. Results are cached. Missing tiles render as dark background, so a
// partial map is returned rather than an error.
func Render(ctx context.Context, x, y, z int) ([]byte, error) {
	key := cacheKey(x, y, z)
	cacheMu.Lock()
	if b, ok := cache[key]; ok {
		cacheMu.Unlock()
		return b, nil
	}
	cacheMu.Unlock()

	b, err := render(ctx, x, y, z)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	if len(cache) >= cacheMax {
		cache = map[string][]byte{} // simple bounded reset
	}
	cache[key] = b
	cacheMu.Unlock()
	return b, nil
}

func render(ctx context.Context, x, y, z int) ([]byte, error) {
	// World rect of the output window, centred on (x, y).
	wx0 := x - windowW/2
	wy0 := y - windowH/2

	// Covering tile range (each tile's top-left is aligned to tileSize).
	tx0 := wx0 / tileSize
	tx1 := (wx0 + windowW - 1) / tileSize
	ty0 := wy0 / tileSize
	ty1 := (wy0 + windowH - 1) / tileSize

	canvas := image.NewRGBA(image.Rect(0, 0, (tx1-tx0+1)*tileSize, (ty1-ty0+1)*tileSize))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{24, 26, 30, 255}), image.Point{}, draw.Src)

	for tx := tx0; tx <= tx1; tx++ {
		for ty := ty0; ty <= ty1; ty++ {
			img, err := fetchTile(ctx, tx*tileSize, ty*tileSize, z)
			if err != nil || img == nil {
				continue // leave dark background for a missing tile
			}
			dst := image.Rect((tx-tx0)*tileSize, (ty-ty0)*tileSize, (tx-tx0+1)*tileSize, (ty-ty0+1)*tileSize)
			draw.Draw(canvas, dst, img, img.Bounds().Min, draw.Src)
		}
	}

	// Crop the window out of the tile-aligned canvas.
	out := image.NewRGBA(image.Rect(0, 0, windowW, windowH))
	draw.Draw(out, out.Bounds(), canvas, image.Point{X: wx0 - tx0*tileSize, Y: wy0 - ty0*tileSize}, draw.Src)

	drawMarker(out, windowW/2, windowH/2)

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fetchTile(ctx context.Context, tileX, tileY, z int) (image.Image, error) {
	url := fmt.Sprintf("%s/Minimap_Color_%d_%d_%d.png", tileBaseURL, tileX, tileY, z)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil // missing tile: not an error
	}
	return png.Decode(resp.Body)
}

// drawMarker paints a simple pin (white-ringed red dot) centred at (cx, cy).
func drawMarker(img *image.RGBA, cx, cy int) {
	white := color.RGBA{255, 255, 255, 255}
	red := color.RGBA{220, 40, 40, 255}
	fillDisc(img, cx, cy, 7, white)
	fillDisc(img, cx, cy, 5, red)
	fillDisc(img, cx, cy, 2, white)
}

func fillDisc(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	r2 := r * r
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(cx+dx, cy+dy, c) // out-of-bounds writes are no-ops
			}
		}
	}
}
