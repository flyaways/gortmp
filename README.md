# GoRTMP 
## Examples:

```golang
package main

import (
	"flag"
	"log"

	"github.com/flyaways/gortmp"
)

var (
	url      = flag.String("i", "rtmp://127.0.0.1:1935/xxx.xxx.xxx.com/live/name", "The rtmp url to connect.")
	fileName = flag.String("f", "", "not empty is publish. empty is push.")
)

func main() {
	flag.Parse()

	client, err := gortmp.New(*url, *fileName)
	if err != nil {
		log.Println(err)

		return
	}

	defer client.Close()

	client.Run()
}

```