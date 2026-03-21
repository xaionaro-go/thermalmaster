package colormap

import "fmt"

// Parse returns a Colormap by name. An empty string or "none" returns (nil, nil).
func Parse(name string) (Colormap, error) {
	switch name {
	case "none", "":
		return nil, nil
	case "inferno":
		return Inferno(), nil
	case "viridis":
		return Viridis(), nil
	case "magma":
		return Magma(), nil
	case "plasma":
		return Plasma(), nil
	case "turbo":
		return Turbo(), nil
	case "cividis":
		return Cividis(), nil
	case "ironbow":
		return Ironbow(), nil
	case "jet":
		return Jet(), nil
	case "whitehot", "white-hot":
		return WhiteHot(), nil
	case "blackhot", "black-hot":
		return BlackHot(), nil
	case "warm":
		return Warm(), nil
	case "cool":
		return Cool(), nil
	case "rainbow":
		return Rainbow(), nil
	case "spectral":
		return Spectral(), nil
	case "reds":
		return Reds(), nil
	case "blues":
		return Blues(), nil
	case "greens":
		return Greens(), nil
	case "greys":
		return Greys(), nil
	case "oranges":
		return Oranges(), nil
	case "purples":
		return Purples(), nil
	case "bugn":
		return BuGn(), nil
	case "bupu":
		return BuPu(), nil
	case "gnbu":
		return GnBu(), nil
	case "orrd":
		return OrRd(), nil
	case "pubu":
		return PuBu(), nil
	case "pubugn":
		return PuBuGn(), nil
	case "purd":
		return PuRd(), nil
	case "rdpu":
		return RdPu(), nil
	case "ylgn":
		return YlGn(), nil
	case "ylgnbu":
		return YlGnBu(), nil
	case "ylorbr":
		return YlOrBr(), nil
	case "ylorrd":
		return YlOrRd(), nil
	case "brbg":
		return BrBG(), nil
	case "prgn":
		return PRGn(), nil
	case "piyg":
		return PiYG(), nil
	case "puor":
		return PuOr(), nil
	case "rdbu":
		return RdBu(), nil
	case "rdgy":
		return RdGy(), nil
	case "rdylbu":
		return RdYlBu(), nil
	case "rdylgn":
		return RdYlGn(), nil
	case "sinebow":
		return Sinebow(), nil
	case "cubehelix":
		return CubehelixDefault(), nil
	default:
		return nil, fmt.Errorf("unknown colormap: %q", name)
	}
}
