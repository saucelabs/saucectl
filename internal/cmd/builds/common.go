package builds

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/tables"
)

func renderListTable(builds []build.Build) {
	if len(builds) == 0 {
		println("Cannot find any builds")
		return
	}

	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"ID", "Name", "Status",
	})

	for _, item := range builds {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			item.ID,
			item.Name,
			item.Status,
		})
	}
	t.SuppressEmptyColumns()
	t.AppendFooter(table.Row{
		fmt.Sprintf("%d builds in total", len(builds)),
	})

	fmt.Println(t.Render())
}
