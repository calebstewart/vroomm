/*
Copyright © 2023 Caleb Stewart

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/calebstewart/vroomm/config"
	"github.com/calebstewart/vroomm/gui"
)

// guiCmd represents the gui command
var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "GTK-based Graphical Virtual Machine Manager",
	Long: `Opens the virtual machine manager as a graphical GTK3-based application.
The application optionally supports wlr-layer-shell protocols through gtk-layer-shell,
and is primarily intended to be used in this way (i.e. as a pop-up VM manager
integrated in your DE as a keyboard shortcut).

The interface is very similar to tools like Dmenu, Rofi or Wofi, and should be
relatively intuitive. Typing will always focus the text input with the exception
of the up/down arrows which select items in the appropriate menu, the enter key
which accepts the current menu or item selection, and escape. Escape alone will
navigate backwards through the menu tree. Pressing Shift+Escape will exit the
application immediately.

This application was built for my own use, and is not heavily tested. If you
don't know what you're doing with low-level libvirt interaction, it is not
recommended to blindly click items in the interface. There are very few
confirmations as the interface assumes you know what you're doing. Good luck.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("layershell.enabled", cmd.Flags().Lookup("layershell"))
		viper.BindPFlag("layershell.width", cmd.Flags().Lookup("layershell-width"))
		viper.BindPFlag("layershell.height", cmd.Flags().Lookup("layershell-height"))
		viper.BindPFlag("use_style", cmd.Flags().Lookup("use-style"))
		viper.BindPFlag("style", cmd.Flags().Lookup("style"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		if cfg, err := config.NewFromViper(); err != nil {
			logrus.WithError(err).Fatal("failed to load configuration")
		} else {
			app := gui.NewApplication(&cfg)
			app.Run(args)
		}
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)

	// Here you will define your flags and configuration settings.
	guiCmd.Flags().Bool("layershell", false, "Enable WLR Layer Shell to create an overlay window")
	guiCmd.Flags().IntP("layershell-width", "W", 0, "Width of the layershell overlay as a percent of the current output")
	guiCmd.Flags().IntP("layershell-height", "H", 0, "Height of the layershell overlay as a percent of the current output")
	guiCmd.Flags().Bool("use-style", false, "Enable loading of GTK stylesheets")
	guiCmd.Flags().String("style", "", "Override the GTK stylesheet search order with an explicit path")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// guiCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// guiCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
