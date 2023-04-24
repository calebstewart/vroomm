/*
The MIT License (MIT)

# Copyright (c) 2023 Caleb Stewart

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package config

import (
	"github.com/spf13/viper"
)

type Edge string

const (
	EdgeNone   Edge = "none"
	EdgeLeft   Edge = "left"
	EdgeRight  Edge = "right"
	EdgeTop    Edge = "top"
	EdgeBottom Edge = "bottom"
)

type LayerShell struct {
	Enabled       bool `mapstructure:"enabled" toml:"enabled"`             // Enable GTK Layer Shell for the GUI window
	Width         int  `mapstructure:"width" toml:"width"`                 // Width of window (in % of output width)
	Height        int  `mapstructure:"height" toml:"height"`               // Height of window (in % of output height)
	Edge          Edge `mapstructure:"edge" toml:"edge"`                   // Which edge of the display to use
	ExclusiveZone int  `mapstructure:"exclusivezone" toml:"exclusivezone"` // Size of the exclusive zone when anchored to an edge (default is auto)
}

// Application Configuration
type Config struct {
	ConnectionString string     `mapstructure:"connect_uri" toml:"connect_uri"`
	LayerShell       LayerShell `mapstructure:"layershell" toml:"layershell"`
	Style            string     `mapstructure:"style" toml:"style"`
	UseStyle         bool       `mapstructure:"use_style" toml:"use_style"`
}

func NewFromViper() (Config, error) {
	cfg := Config{
		ConnectionString: "qemu:///system",
		UseStyle:         true,
		Style:            "",
	}

	return cfg, viper.Unmarshal(&cfg)
}
