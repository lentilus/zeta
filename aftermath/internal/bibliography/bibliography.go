package bibliography

import (
	"aftermath/internal/cache/database"
	"aftermath/internal/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Bibliography represents the bibliography management system.
type Bibliography struct {
	Path     string
	DB       *database.DB
	Checksum []byte
}

// Zettel2Entry converts a Zettel to a yaml entry.
func Zettel2Yaml(z database.Zettel, root string) string {
	target, _ := utils.Path2Target(z.Path, root)
	path, _ := filepath.Rel(root, z.Path)
	template := `"%s":
  type: Misc
  title: "%s"
  path: "%s"
  id: %s
`

	return fmt.Sprintf(template, target, target, path, fmt.Sprint(z.ID))
}

// Regenerate writes the entire bibliography to the YAML file.
func (b *Bibliography) Regenerate(root string) error {
	log.Println("Regenerating Bibliography")
	zettels, err := b.DB.GetAllSorted()
	if err != nil {
		return err
	}

	yamlData := ``
	for _, z := range zettels {
		yamlData += Zettel2Yaml(z, root)
	}

	return os.WriteFile(b.Path, []byte(yamlData), 0644)
}
