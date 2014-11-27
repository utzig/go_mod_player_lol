package main

import (
	"code.google.com/p/portaudio-go/portaudio"
	"os"
	"fmt"
	//"encoding/binary"
	//"time"
	"os/signal"
)

const (
	PAL_CLOCK          = 7093789.2
	SAMPLE_RATE        = 44100
	MAX_PATTERNS       = 128
	MAX_SAMPLES        = 31
	MAX_DIVISIONS      = 64
	MAX_CHANNELS       = 4
	DIVISION_LEN       = 16
)

/*
const (
	C1  = 856,
	C_1 = 808,
	D1  = 762,
	D_1 = 720,
	E1  = 678,
	F1  = 640,
	F_1 = 604,
	G1  = 570,
	G_1 = 538,
	A1  = 508,
	A_1 = 480,
	B1  = 453,
	C2  = 856,
	C_2 = 808,
	D2  = 762,
	D_2 = 720,
	E2  = 678,
	F2  = 640,
	F_2 = 604,
	G2  = 570,
	G_2 = 538,
	A2  = 508,
	A_2 = 480,
	B2  = 453,
)
*/

type sample struct {
	name string
	length uint16 // length in words
	volume uint8  // 1-64
	repeatoff uint16
	repeatlen uint16
	data []byte
}

type channel struct {
	sample byte
	period uint16
	effect uint16
}

// channel 1+4 go to LEFT and 2+3 go to RIGHT
type division struct {
	channels [4]channel
}

type pattern struct {
	divisions [MAX_DIVISIONS]division
	initialized bool
}

type module struct {
	title string
	samples [MAX_SAMPLES]sample
	patterns [MAX_PATTERNS]pattern
	positions int
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

	out := make([]byte, 512)

	stream, err := portaudio.OpenDefaultStream(0, 1, SAMPLE_RATE, len(out), &out)
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		panic(err)
	}
	defer stream.Stop()

	//XXX: debug code
	for ch := 0; ch < MAX_CHANNELS; ch++ {
		for div := 0; div < MAX_DIVISIONS; div++ {
			fmt.Print("[", m.patterns[39].divisions[div].channels[ch].sample, "]")
		}
		fmt.Println()
	}


	/*
	var offset float32 = 0.0
	for i := 0; i < MAX_SAMPLES; i++ {
		if m.samples[i].length > 0 {
			for {
				offset = bufferFromSample(out, m.samples[i].data, offset)
				err = stream.Write();
				if err != nil {
					panic(err)
				}
				select {
				case <-sig:
					return
				default:
				}
				if offset == 0.0 {
					break
				}
			}
		}
	}
	*/
}

func bufferFromSample(out []byte, in []byte, offset float32) float32 {
	var palRate float32 = PAL_CLOCK / (428 * 2)
	var rate float32 = palRate / float32(SAMPLE_RATE)
	var index, last_index int
	var y0, y1 byte
	var x0 float32

	inLen := len(in)
	outLen := len(out)

	index = int(offset)
	last_index = index
	y0 = in[index]
	if index == (inLen - 1) {
		y1 = in[index]
	} else {
		y1 = in[index + 1]
	}
	x0 = offset
	for i := 0; i < outLen; i++ {
		if index == (inLen - 1) {
			for j := i; j < outLen; j++ {
				out[j] = 0
			}
			return 0.0
		}
		if last_index != index {
			out[i] = in[index]
			last_index = index
			y0, y1 = in[index], in[index+1]
			x0 = offset
		} else {
			// linear interpolation
			out[i] = y0 + (y1 - y0) * byte((offset - x0) / palRate)
		}
		offset += rate
		index = int(offset)
	}

	return offset
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

	buffer = make([]byte, 30 * MAX_SAMPLES)
	n, err = file.Read(buffer)
	if err != nil {
		return m, err
	}

	total += n

	for i := 0; i < MAX_SAMPLES; i++ {
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

	// M.K. used to check if has 15 or 31 samples
	fmt.Println(string(buffer[130:134]))

	fmt.Println("total: ", total)

	for i := 0; i < MAX_PATTERNS; i++ {
		m.patterns[i].initialized = false
	}

	/* Now comes the pattern data */

	patterns := make([]byte, MAX_PATTERNS)
	copy(patterns, buffer[2:130])

	p := 0
	buffer = make([]byte, 1024)
	for i := 0; i < m.positions; i++ {
		p = int(patterns[i])
		fmt.Print("[", p, "]")
		if m.patterns[p].initialized == false {
			n, err = file.Read(buffer)
			if err != nil {
				return m, err
			}
			loadPattern(buffer, &m.patterns[p])
			total += n
		}
	}
	fmt.Println()
	fmt.Println("total: ", total)

	// Sample data
	for i := 0; i < MAX_SAMPLES; i++ {
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

func loadPattern(b []byte, p *pattern) {
	var sample byte
	var period, effect uint16

	for j := 0; j < MAX_DIVISIONS; j++ {
		for c := 0; c < MAX_CHANNELS; c++ {
			sample  = b[j*DIVISION_LEN + c*MAX_CHANNELS + 2] >> 4
			sample |= b[j*DIVISION_LEN + c*MAX_CHANNELS + 0] & 0xF0

			period  = uint16(b[j*DIVISION_LEN + c*MAX_CHANNELS + 1])
			period |= uint16(b[j*DIVISION_LEN + c*MAX_CHANNELS + 0] & 0x0F) << 8

			effect  = uint16(b[j*DIVISION_LEN + c*MAX_CHANNELS + 3])
			effect |= uint16(b[j*DIVISION_LEN + c*MAX_CHANNELS + 2] & 0x0F) << 8

			p.divisions[j].channels[c].sample = sample
			p.divisions[j].channels[c].period = period
			p.divisions[j].channels[c].effect = effect
		}
	}
	p.initialized = true
}
