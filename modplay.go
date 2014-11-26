package main

import (
	"code.google.com/p/portaudio-go/portaudio"
	"os"
	"fmt"
	//"encoding/binary"
	//"time"
	"os/signal"
)

type sample struct {
	name string
	length uint16 // length in words
	volume uint8  // 1-64
	repeatoff uint16
	repeatlen uint16
	data []byte
}

type pattern struct {
	data []byte
}

type module struct {
	title string
	samples [31]sample
	patterns [128]pattern
	positions int
}

func loadModule(name string) (module, error) {
	var file *os.File
	var n int
	var m module
	var err error
	var buffer []byte

	file, err = os.Open(name)
	if err != nil {
		return m, err
	}

	total := 0

	buffer = make([]byte, 20)
	n, err = file.Read(buffer)
	if err != nil {
		return m, err
	}

	total += n

	m.title = string(buffer)
	fmt.Println(m.title)

	//TODO: determine number of samples by reading at offsets
	//      600 (15 samples) and 1080 (31 samples) and looking
	//      for M.K.

	buffer = make([]byte, 30 * 31)
	n, err = file.Read(buffer)
	if err != nil {
		return m, err
	}

	total += n

	for i := 0; i < 31; i++ {
		m.samples[i].name = string(buffer[i*30:i*30+22])
		//fmt.Println(m.samples[i].name)
		m.samples[i].length = uint16(buffer[i*30+22]) << 8 + uint16(buffer[i*30+23])
		//fmt.Println(m.samples[i].length * 2)
		m.samples[i].volume = buffer[i*30+25]
		//fmt.Println(m.samples[i].volume)
		m.samples[i].repeatoff = uint16(buffer[i*30+26]) << 8 + uint16(buffer[i*30+27])
		m.samples[i].repeatlen = uint16(buffer[i*30+28]) << 8 + uint16(buffer[i*30+29])
	}

	buffer = make([]byte, 134)
	n, err = file.Read(buffer)
	if err != nil {
		return m, err
	}

	total += n

	// number of song positions
	m.positions = int(buffer[0])
	fmt.Println(m.positions)

	// TODO: pattern table is at buffer[2:130]

	// M.K. used to check if has 15 or 31 samples
	fmt.Println(string(buffer[130:134]))

	fmt.Println("total: ", total)

	// TODO: Pattern data
	pattern := 0
	patternAmount := 0
	for i := 0; i < m.positions; i++ {
		pattern = int(buffer[2+i])
		fmt.Print("[", pattern, "]")
		if m.patterns[pattern].data == nil {
			m.patterns[pattern].data = make([]byte, 1024)
			n, err = file.Read(m.patterns[pattern].data)
			if err != nil {
				return m, err
			}
			total += n
			patternAmount++
		}
	}
	fmt.Println()
	fmt.Println("total: ", total)
	fmt.Println("patternAmount: ", patternAmount)

	// Sample data
	for i := 0; i < 31; i++ {
		if m.samples[i].length > 0 {
			lenby := m.samples[i].length * 2
			//fmt.Println("Sample length: ", lenby)
			m.samples[i].data = make([]byte, lenby)
			n, err = file.Read(m.samples[i].data)
			total += n
			if err != nil {
				return m, err
			}
		}
	}

	fmt.Println("total: ", total)

	return m, nil
}

func main() {
	var m module

	if len(os.Args) < 2 {
		fmt.Println("Missing mod file name")
		return
	}
	fmt.Println("Playing. Press Ctrl-C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	err := portaudio.Initialize()
	if err != nil {
		panic(err)
	}
	defer portaudio.Terminate()

	m, err = loadModule(os.Args[1])
	if err != nil {
		panic(err)
	}

	out := make([]byte, 128)

	stream, err := portaudio.OpenDefaultStream(0, 1, 22050, len(out), &out)
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		panic(err)
	}
	defer stream.Stop()

	var offset float32 = 0.0
	for /*i := 0; i < 31; i++*/ {
		//if m.samples[i].length > 0 {
			offset = bufferFromSample(out, m.samples[10].data, offset)
			err = stream.Write();
			if err != nil {
				panic(err)
			}
			select {
			case <-sig:
				return
			default:
			}
		//}
	}
}

func bufferFromSample(out []byte, in []byte, offset float32) float32 {
	// with pitch C2, 8287 bytes of sampled data are sent to
	// channel per second. Channel is configure at 22050
	var sampleRate float32 = 8287.0 / 22050.0
	inLen := len(in)
	outLen := len(out)

	for i := 0; i < outLen; i++ {
		out[i] = in[int(offset) % inLen]
		offset += sampleRate
	}

	return offset
}
