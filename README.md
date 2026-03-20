# linx-client

Simple CLI for [linx-server](https://github.com/andreimarcu/linx-server) ([active fork](http://github.com/gabe565/linx-server)) that keeps a JSON logfile of deletion keys so you can revisit uploads later.

> This is an active fork of the [official client](https://github.com/andreimarcu/linx-client)

## Features

- Upload files from disk or stdin and record the deletion key
- Overwrite or delete uploads using the stored deletion key
- Compare client-side SHA256 with the server copy after every upload
- Inspect (`-ls`) or clean (`-cleanup`) the logfile without digging through JSON by hand

## Get a release

1. Grab the latest binary from the [releases](https://github.com/Lanjelin/linx-client/releases) (or build locally with `go build -o linx-client`).
2. Run `./linx-client …`.

## Configuration

On first run the CLI prompts for the Linx instance URL, logfile path, and optional API key. Use `-c /path/to/linx-client.conf` to load a different config file.

```sh
$ ./linx-client  
Configuring linx-client  
  
Site url (ex: https://linx.example.com/): https://linx.example.com/  
Logfile path (ex: ~/.linxlog): ~/.linxlog  
API key (leave blank if instance is public):  
  
Configuration written at ~/.config/linx-client.conf
```

## Usage

### Upload file(s)

```sh
$ linx-client path/to/file.ext
https://linx.example.com/file.ext
```

### Options

- `-f file.ext` — force a specific filename (used for stdin uploads too)
- `-r` — randomize the filename on the server
- `-e 600` — set the expiry time in seconds
- `-deletekey mysecret` — provide your own deletion key for the upload(s)
- `-accesskey mykey` — attach an access password to the upload
- `-c myconfig.json` — use a different config file (creates it if missing)
- `-no-cb` — do not copy the resulting URL to your clipboard
- `-selif` — print the server’s direct Selif URL (useful for short links)
- `-o` — overwrite an existing upload using its stored deletion key
- `-d` — delete URL(s) listed on the command line
- `-cleanup` — drop entries from the logfile whose URLs return 404/410
- `-ls` — list every logged URL and its associated deletion key

### Upload from stdin

```sh
$ echo "hello there" | linx-client -
https://linx.example.com/random.txt
```

Add `-f hello.txt` if you want to control the filename.

### Overwrite file

If you previously uploaded `file.ext` and saved its deletion key, you can run:

```sh
$ linx-client -o file.ext
https://linx.example.com/file.ext
```

### Delete file(s)

```sh
$ linx-client -d https://linx.example.com/file.ext
Deleted https://linx.example.com/file.ext
```

### Logfile helpers

- `-ls` prints every tracked upload with its deletion key so you can audit, copy keys, or follow up from the terminal.
- `-cleanup` iterates over the saved URLs and removes the ones that now respond with 404/410 before writing the reduced logfile back to disk.

## License
```
License
-------
Copyright (C) 2015 Andrei Marcu

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

Author
---
Andrei Marcu, http://andreim.net/
```
```
