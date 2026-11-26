package docpdf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Fonts holds the four styles of the serif family the documents are set in. Missing bold or
// italic files fall back to the regular face.
type Fonts struct {
	Family string
	Source string
	faces  map[string][]byte // "", "B", "I", "BI"
}

// ErrNoFont means no Times New Roman (or Liberation Serif) font could be found.
var ErrNoFont = errors.New("không tìm thấy font Times New Roman hoặc Liberation Serif cho PDF")

type family struct {
	name  string
	files map[string][]string // style → candidate file names (lower case)
}

// Times New Roman first; Liberation Serif has the same metrics and covers Vietnamese, so a
// server without Times New Roman lays the documents out the same way.
var families = []family{
	{"Times New Roman", map[string][]string{
		"":   {"times new roman.ttf", "times.ttf", "times_new_roman.ttf", "timesnewroman.ttf"},
		"B":  {"times new roman bold.ttf", "timesbd.ttf", "times_new_roman_bold.ttf", "timesnewromanbold.ttf"},
		"I":  {"times new roman italic.ttf", "timesi.ttf", "times_new_roman_italic.ttf", "timesnewromanitalic.ttf"},
		"BI": {"times new roman bold italic.ttf", "timesbi.ttf", "times_new_roman_bold_italic.ttf", "timesnewromanbolditalic.ttf"},
	}},
	{"Liberation Serif", map[string][]string{
		"":   {"liberationserif-regular.ttf"},
		"B":  {"liberationserif-bold.ttf"},
		"I":  {"liberationserif-italic.ttf"},
		"BI": {"liberationserif-bolditalic.ttf"},
	}},
}

// systemFontDirs are searched (recursively) after the configured directory.
var systemFontDirs = []string{
	"/System/Library/Fonts/Supplemental", "/Library/Fonts", // macOS
	`C:\Windows\Fonts`, // Windows
	"/usr/share/fonts", // Linux: msttcorefonts, font-liberation…
	"/usr/local/share/fonts",
}

// LoadFonts finds the document font: Times New Roman in dir (PDF_FONT_DIR) or the system
// font folders, else Liberation Serif. Within a family, earlier folders win and all styles come
// from the folder that has the regular face.
func LoadFonts(dir string) (*Fonts, error) {
	dirs := systemFontDirs
	if dir != "" {
		dirs = append([]string{dir}, dirs...)
	}
	indexes := make([]map[string]string, len(dirs))
	for i, d := range dirs {
		indexes[i] = indexFonts([]string{d})
	}
	for _, fam := range families {
		for i, index := range indexes {
			regular := find(index, fam.files[""])
			if regular == "" {
				continue
			}
			f := &Fonts{Family: fam.name, Source: dirs[i], faces: map[string][]byte{}}
			for style, names := range fam.files {
				path := find(index, names)
				if path == "" {
					path = regular
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return nil, fmt.Errorf("đọc font %s: %w", path, err)
				}
				f.faces[style] = b
			}
			return f, nil
		}
	}
	return nil, ErrNoFont
}

// indexFonts maps lower-case .ttf file names to the first path found, earlier dirs winning.
func indexFonts(dirs []string) map[string]string {
	index := map[string]string{}
	for _, root := range dirs {
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			continue
		}
		depth := strings.Count(filepath.Clean(root), string(filepath.Separator))
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.Count(path, string(filepath.Separator))-depth > 4 {
					return filepath.SkipDir
				}
				return nil
			}
			name := strings.ToLower(d.Name())
			if strings.HasSuffix(name, ".ttf") {
				if _, ok := index[name]; !ok {
					index[name] = path
				}
			}
			return nil
		})
	}
	return index
}

func find(index map[string]string, names []string) string {
	for _, n := range names {
		if p, ok := index[n]; ok {
			return p
		}
	}
	return ""
}
