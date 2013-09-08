// parsrv is a simple server for parget
// capable of serving multiple concurrent downloads
// type parsrv -h to see available options
package main

import (
	"flag"
	"github.com/captaincronos/parget"
	"io"
	"log"
	"net"
	"os"
)

var (
	listen_addr = flag.String("a", ":8080", "address to listen on")
	serve_path  = flag.String("p", ".", "path with files to be served")
)

func main() {
	flag.Parse()

	l, err := net.Listen("tcp", *listen_addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		c, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go serve(c)
	}
}

func serve(c net.Conn) {
	defer c.Close()

	var curf *os.File // keeps requested file opened to avoid multiple open/closes
	var curp string   // path to he currently opened path

	defer func() {
		if curf != nil {
			curf.Close()
		}
	}()

	for {
		m, err := parget.Read(c)
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Println("failed to read the message:", err)
			break
		}

		resp := parget.Msg{Typ: parget.TypErr}

		// if file is not yet opened or a different file is requested, open it up
		if curf == nil || curp != m.Path {
			if curf != nil {
				curf.Close()
			}
			curf, err = os.OpenFile(m.Path, os.O_RDONLY, 0)
			curp = m.Path
			if err != nil {
				resp.Typ = parget.TypErr
				resp.Data = []byte(err.Error())
				log.Println("failed to open file", m.Path, err)
			}
		}

		switch m.Typ {
		case parget.TypGetLen:
			log.Println(m.Path, "requested")
			stat, err := curf.Stat()
			if err != nil {
				resp.Data = []byte(err.Error())
				log.Println(curp, "Stat failed:", err)
				break
			}
			resp.Typ = parget.TypLen
			resp.Len = stat.Size()
		case parget.TypGet:
			_, err = curf.Seek(m.Offset, 0)
			if err != nil {
				resp.Data = []byte(err.Error())
				log.Println(curp, "Seek failed:", err)
				break
			}
			chunk := make([]byte, m.Len)
			_, err := io.ReadFull(curf, chunk)
			// clients should not "accidentaly" jump over file size
			// if that happens, it's considered to be client's problem
			if err != nil {
				resp.Data = []byte(err.Error())
				log.Println(curp, "failed to read:", err)
				break
			}
			resp.Typ = parget.TypData
			resp.Data = chunk
		default:
			log.Println("invalid message type from client:", m.Typ)
			resp.Typ = parget.TypErr
			resp.Data = []byte("invalid message type")
		}
		err = parget.Write(c, resp)
		if err != nil {
			log.Println("failed to write message:", err)
			break
		}
	}
}
