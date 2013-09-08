// pgcl is a client for parget
// it only downloads one single file per run
// TODO: add chunks that have failed to load back into the queue
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/captaincronos/parget"
)

var usage = `Usage: parcl <server_addr> <remote_file_name> <local_file_name> [-chunk chunk_size] [-conns num_cons]`
var chunk_size int64 = 1 << 20 // 1MB
var num_conns = 5              // 5 concurrent connections (5 is a great number!)

func main() {
	if len(os.Args) < 4 {
		fmt.Println(usage)
		os.Exit(1)
	}

	addr := os.Args[1]
	rpath := os.Args[2]
	lpath := os.Args[3]

	// additional flags specified
	if len(os.Args) > 4 {
		parse_flags(os.Args[4:])
	}

	// reqest file length from server
	flen, err := get_len(addr, rpath)
	if err != nil {
		fmt.Println("failed to get length of file", err)
		os.Exit(1)
	}
	if flen == 0 {
		fmt.Println("server reported file size 0")
		os.Exit(1)
	}

	fmt.Println("downloading", rpath, "of size", flen)
	fmt.Println("chunk size:", chunk_size, "concurrent downloads:", num_conns)

	dl_ch := make(chan *chunk)             // channel of chunks to download
	mrg_ch := make(chan *chunk, num_conns) // channel of downloaded chunks to be written to the file
	dl_wg := &sync.WaitGroup{}             // for downloaders
	mrg_wg := &sync.WaitGroup{}            // for merger (maybe a channel is simpler?)

	mrg_wg.Add(1)
	go merge(lpath, flen, mrg_ch, mrg_wg)

	for i := 0; i < num_conns; i++ {
		dl_wg.Add(1)
		go download(addr, rpath, dl_ch, mrg_ch, dl_wg)
	}

	// give ranges to be downloaded
	// each range is at most chunk_size
	for i := int64(0); i < flen; i += chunk_size {
		dl_ch <- &chunk{off: i, len: min(chunk_size, flen-i)}
	}

	// break range loops in all loaders
	close(dl_ch)
	dl_wg.Wait()

	// break loop in merger to make it flush/close file
	close(mrg_ch)
	mrg_wg.Wait()
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// parses optional -chunk and -conns flags
func parse_flags(flags []string) {
	f := flag.NewFlagSet("params", flag.ExitOnError)
	cs := f.Int64("chunk", chunk_size, "size of chunks being downloaded")
	nc := f.Int("conns", num_conns, "number of concurrent connections")
	f.Usage = func() { fmt.Println(usage) }
	f.Parse(flags)
	if cs != nil {
		chunk_size = *cs
	}
	if nc != nil {
		num_conns = *nc
	}
}

// get_len sends request to the server to determine length of file to download
func get_len(addr, path string) (int64, error) {
	con, err := net.Dial("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer con.Close()

	if err = parget.Write(con, parget.Msg{Typ: parget.TypGetLen, Path: path}); err != nil {
		return 0, err
	}

	resp, err := parget.Read(con)
	if err != nil {
		return 0, err
	}
	if resp.Typ == parget.TypErr {
		return 0, errors.New("error message from server: " + string(resp.Data))
	}
	if resp.Typ != parget.TypLen {
		return 0, errors.New("invalid response type from server: " + strconv.Itoa(int(resp.Typ)))
	}
	return resp.Len, nil
}

// chunk represents a piece of file being downloaded
type chunk struct {
	off, len int64
	data     []byte
}

// download gets chunk info from in channel, downloads corresponding chunk and sends it to out channel
func download(addr, path string, in, out chan *chunk, wg *sync.WaitGroup) {
	defer wg.Done()
	var con net.Conn
	var err error

	for ch := range in {
		if con == nil {
			con, err = net.Dial("tcp", addr)
			if err != nil {
				log.Println("failed to connect to the server:", err)
				return
			}
			defer con.Close()
		}
		err = parget.Write(con, parget.Msg{Typ: parget.TypGet, Offset: ch.off, Len: ch.len, Path: path})
		if err != nil {
			log.Println("failed to write the message:", err)
			continue
		}
		m, err := parget.Read(con)
		if err != nil {
			log.Println("failed to read the message:", err)
			continue
		}
		if m.Typ == parget.TypErr {
			log.Println("error message from server:", string(m.Data))
			continue
		}
		if m.Typ != parget.TypData {
			log.Println("invalid message type from server", m.Typ)
			continue
		}
		ch.data = m.Data

		out <- ch
	}
}

// merge creates a file on path, then reads chunks from in channel and writes to corresponding parts of file
func merge(path string, size int64, in chan *chunk, wg *sync.WaitGroup) {
	defer wg.Done()
	var total int64
	f, err := os.Create(path)
	if err != nil {
		log.Fatalln(err)
	}
	// make sure we have exactly the amount of space we need
	if err = f.Truncate(size); err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	for ch := range in {
		_, err = f.Seek(ch.off, 0)
		if err != nil { // downloaded too much?
			log.Println(err)
			continue
		}
		n, err := f.Write(ch.data)
		if err != nil {
			log.Println(err)
			continue
		}
		total += int64(n)
		fmt.Print("\r", total, "/", size)
	}
}
