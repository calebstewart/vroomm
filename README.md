# Vroomm - Virtual Machine Manager

https://user-images.githubusercontent.com/7529189/233879555-825fd8ee-4427-430a-b1f9-cf6c49a78459.mp4

Vroomm is a simple virtual machine manager for `libvirt`. The motivation for
this project was the relatively negative view I have towards `virt-manager`.
The stated goals of `virt-manager` is not to be user-friendly, but to be
stable and therefore does not support some nicer features like linked
(read: copy-on-write) clones or VM organization of any kind.

`virt-manager` directs users to other `libvirt` interfaces for "fancier"
features, but none currently exist. The closest is Gnome Boxes, but I am
not a Gnome user, and it still does not provide useful organization
capabilities.

Vroomm attempts to solve my gripes with the current interfaces by
creating an interactive management utility which integrates well with
tiling window managers and menu systems like Dmenu or Rofi.

Currently supported features:
* Simple Dmenu-like interface focused on keyboard input
* `wlr-layer-shell` support for an integrated menu feel
* Browse/filter VMs by name
* Organize and browse VMs inside a pseudo-filesystem
* Organize and browse VMs with arbitrary tags/lables
* Create linked and full clones interactively
* Edit and apply changes to raw libvirt domain XML
* Start `virt-viewer` or `looking-glass` for VMs.

Features In Progress:
* Manage snapshots (create, restore, delete)
* Interactively move VMs inside the pseudo-filesystem
* Interactively add tags/labels to VMs
* View "child" VMs which exist as clones of a single VM

## Usage
The package builds a single binary. I hope to eventually provide a
terminal-based UI, so the main GUI interface is available with the
subcommand `gui` like this:

``` sh
./vroomm gui
```

There are a few command line arguments which include the `libvirt`
connection string, which allows you to connect to remote `libvirt`
services, the configuration file location, and `wlr-layer-shell`
related options.

All command line options can also be specified in a configuration file.
See [./example-config.toml] for example configuration. The configuration
file will be loaded from `$XDG_CONFIG_HOME/vroomm/config.toml` if it
exists. Command line arguments take prescedence over configuration file
settings.

## Editing XML
Vroomm has the ability to open a domain XML description in a text
editor to update VM properties. This is accomplished by saving the
domain XML to a local temporary file, and then opening it with `xdg-open`.
If Vroomm is opening the wrong editor, then you need to configure
your default applications in whatever way makes sense for your
environment (in general, that is `~/.config/mimeapps.list`, but may
vary by distribution).

## Styling
By default, styling is disabled and the window will inherit your GTK
theme. This may work fine for you, but you can also load custom CSS.
There is a CSS file embedded in the executable which provides a dark
theme. The `--use-style` argument will cause the GUI application to
load a CSS stylesheet. By default, the application will look in all
XDG configuration directories, and load the first `vroomm/style.css`
file it finds (checking `XDG_CONFIG_HOME` first, then `XDG_CONFIG_DIRS`).
You can also optionally specify a custom CSS stylesheet path with
the `--style` option or the `style` field in the configuration file.
This path will override the default search paths and the builtin
stylesheet.

If any stylesheet in the search order cannot be found or is not valid,
the search will continue. This means that if you enable CSS stylesheets,
and your stylesheet is invalid (and all others in the search path are
invalid), the embedded stylesheet will be used. Errors regarding
stylesheet loading are logged to `stderr`.

## Disclaimer
This project was written for myself. It is not a supported product and
is not heavily tested except by me. Given that it interacts with your
hypervisor at an API level, you should be wary of clicking buttons or
generally using this application unless you know what you're doing.
While I've attempted to handle all errors gracefully, this application
makes no guarantees or promises to be 100% correct all the time. If
you're working with important VMs or data, and you aren't able to at
least read through and understand the APIs that make this work, then
I'd recommend not using it.

That being said, if you're just tinkering around with your personal VMs
and want to give it a go, that's awesome. If you find any odd errors,
please let me know through an issue here. I probably just haven't tested
the case you're running into. :)
