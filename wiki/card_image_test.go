package wiki

import (
	"image"
	"image/color"
	"testing"
)

func TestStarCostFromCardImage(t *testing.T) {
	tests := []struct {
		name string
		star bool
		want string
	}{
		{name: "普通卡牌", want: ""},
		{name: "消耗辉星", star: true, want: "2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := image.NewNRGBA(image.Rect(0, 0, 240, 300))
			if tt.star {
				for y := 40; y < 55; y++ {
					for x := 3; x < 15; x++ {
						card.Set(x, y, color.NRGBA{R: 45, G: 210, B: 255, A: 255})
					}
				}
			}
			if got := starCostFromCardImage(card); got != tt.want {
				t.Fatalf("starCostFromCardImage() = %q，期望 %q", got, tt.want)
			}
		})
	}
}
