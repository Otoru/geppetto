package profile_test

import (
	"embed"
	"fmt"

	"github.com/vitorhugo/geppetto/profile"
)

// profiles holds the profile documents shipped inside the game binary, the
// way a real game embeds its content.
//
//go:embed testdata/villager.json
var profiles embed.FS

// A game embeds its profile documents and loads them at startup; a document
// the loader rejects fails before the simulation runs.
func ExampleLoadFS() {
	loaded, err := profile.LoadFS(profiles, "testdata/villager.json")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s: %d consideration\n", loaded.Name, len(loaded.Considerations))

	// Output:
	// villager: 1 consideration
}
