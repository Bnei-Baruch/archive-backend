package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
	"io/ioutil"
)

var outFile string

const template = `# mdb2es generated config template
[mdb]
url="postgres://localhost/mdb?sslmode=disable"

[kmedia]
url="postgres://localhost/kmedia?sslmode=disable"

[elasticsearch]
url="http://127.0.0.1:9200"

[server]
bind-address=":8080"
`

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate configuration file template",
	Long: "Write default configuration to given file or stdout",
	Run: configFn,
}

func init() {
	RootCmd.AddCommand(configCmd)
	configCmd.Flags().StringVarP(&outFile, "file", "f", "", "Path to generated config file (default is config.toml)")
}

func configFn(cmd *cobra.Command, args []string) {
	if outFile == "" && len(args) > 0 {
		outFile = args[0]
	}
	if outFile == "" {
		fmt.Print(template)
	} else {
		if err := ioutil.WriteFile(outFile, []byte(template), 0644); err != nil {
			panic(err)
		}
	}
}
