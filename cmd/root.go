/*
Copyright © 2021 Edmond Cotterell

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/Daskott/kronus/version"
	"github.com/spf13/cobra"
)

var (
	isDevEnv bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd *cobra.Command

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd = createRootCmd()
	rootCmd.Version = fmt.Sprintf("v%s", version.Version)
}

func createRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kronus",
		Short: `kronus is a CLI App that lets you run liveliness probes
for users on its server.`,
		ValidArgs: []string{"server"},
	}

	cmd.PersistentFlags().BoolVarP(&isDevEnv, "dev", "", false, "run in development mode")

	cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	return cmd
}
