package inventory

import (
	"fmt"
	"log"
	"os"
	"text/template"

	"github.com/Berops/platform/proto/pb"
)

// Generate generates a new Ansible inventory file
func Generate(nodes []*pb.Node) {
	fmt.Println(nodes)

	tpl, err := template.ParseFiles("./inventory/inventory.goini")
	if err != nil {
		log.Fatalln("Failed to load template file", err)
	}

	f, err := os.Create("inventory/inventory.ini")
	if err != nil {
		log.Fatalln("Failed to create a inventory file", err)
	}

	err = tpl.Execute(f, nodes)
	if err != nil {
		log.Fatalln("Failed to execute template file", err)
	}
}
