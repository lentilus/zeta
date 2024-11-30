package bibliography

import (
	"aftermath/internal/database"
	"fmt"
	"os"
)

// Bibliography represents the bibliography management system.
type Bibliography struct {
	Path     string
	DB       *database.DB
	Checksum []byte
}

// Zettel2Entry converts a Zettel to a yaml entry.
func Zettel2Yaml(z database.Zettel) string {
	template := `"%s":
  type: Misc
  title: "%s"
  path: "%s"
  id: %s
`
	return fmt.Sprintf(template, z.Path, "Zettel: "+z.Path, z.Path, fmt.Sprint(z.ID))
}

// Regenerate writes the entire bibliography to the YAML file.
func (b *Bibliography) Regenerate() error {
	zettels, err := b.DB.GetAllSorted()
	if err != nil {
		return err
	}

	yamlData := ``
	for _, z := range zettels {
		yamlData += Zettel2Yaml(z)
	}

	return os.WriteFile(b.Path, []byte(yamlData), 0644)
}
