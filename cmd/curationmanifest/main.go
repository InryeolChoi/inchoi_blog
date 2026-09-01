// curationmanifest emits the SQL allow-list used immediately before a DB upload.
//
// The upload guard normally rejects every row missing from the new database.
// DropPosts and DropImages are the one intentional exception: their stable IDs
// live in internal/curation, so the allow-list must be generated from there
// rather than copied into a deployment script.
package main

import (
	"fmt"
	"strings"

	"github.com/inryeol/blog/internal/curation"
)

func quoteSQL(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func main() {
	fmt.Println("CREATE TEMP TABLE intentional_dropped_posts (notion_page_id TEXT PRIMARY KEY);")
	for _, post := range curation.DropPosts {
		fmt.Printf("INSERT INTO intentional_dropped_posts VALUES (%s);\n", quoteSQL(post.NotionPageID))
	}

	fmt.Println("CREATE TEMP TABLE intentional_dropped_images (sha256 TEXT PRIMARY KEY);")
	for _, image := range curation.DropImages {
		fmt.Printf("INSERT INTO intentional_dropped_images VALUES (%s);\n", quoteSQL(image.SHA256))
	}
}
