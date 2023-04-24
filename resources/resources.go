package resources

import "embed"

//go:embed main-window.glade style.css
var ResourceFS embed.FS

func Read(path string) ([]byte, error) {
	return ResourceFS.ReadFile(path)
}
