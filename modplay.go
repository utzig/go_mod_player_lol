package main

import (
	"code.google.com/p/portaudio-go/portaudio"
	"os"
	"fmt"
	//"encoding/binary"
	"time"
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

var notes = map[uint16]string {
	856:  "C-1",
	808:  "C#1",
	762:  "D-1",
	720:  "D#1",
	678:  "E-1",
	640:  "F-1",
	604:  "F#1",
	570:  "G-1",
	538:  "G#1",
	508:  "A-1",
	480:  "A#1",
	453:  "B-1",
	428:  "C-2",
	404:  "C#2",
	381:  "D-2",
	360:  "D#2",
	339:  "E-2",
	320:  "F-2",
	302:  "F#2",
	285:  "G-2",
	269:  "G#2",
	254:  "A-2",
	240:  "A#2",
	226:  "B-2",
	214:  "C-3",
	202:  "C#3",
	190:  "D-3",
	180:  "D#3",
	170:  "E-3",
	160:  "F-3",
	151:  "F#3",
	143:  "G-3",
	135:  "G#3",
	127:  "A-3",
	120:  "A#3",
	113:  "B-3",
}

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
	table []byte
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
	go func() {
		select {
		case <-sig:
			os.Exit(0)
		}
	}()

	err := portaudio.Initialize()
	if err != nil {
		panic(err)
	}
	defer portaudio.Terminate()

	err = m.load(os.Args[1])
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

	m.play()

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

func (m *module) load(name string) error {
	var file *os.File
	var n int
	var err error
	var buffer []byte

	file, err = os.Open(name)
	if err != nil {
		return err
	}

	total := 0

	buffer = make([]byte, 20)
	n, err = file.Read(buffer)
	if err != nil {
		return err
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
		return err
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
		return err
	}

	total += n

	// number of song positions
	m.positions = int(buffer[0])

	// M.K. used to check if has 15 or 31 samples
	//fmt.Println(string(buffer[130:134]))

	for i := 0; i < MAX_PATTERNS; i++ {
		m.patterns[i].initialized = false
	}

	/* Now comes the pattern data */

	m.table = make([]byte, MAX_PATTERNS)
	copy(m.table, buffer[2:130])

	p := 0
	buffer = make([]byte, 1024)
	for i := 0; i < m.positions; i++ {
		p = int(m.table[i])
		//fmt.Print("[", p, "]")
		if m.patterns[p].initialized == false {
			n, err = file.Read(buffer)
			if err != nil {
				return err
			}
			loadPattern(buffer, &m.patterns[p])
			total += n
		}
	}
	//fmt.Println()
	//fmt.Println("total: ", total)

	// Sample data
	for i := 0; i < MAX_SAMPLES; i++ {
		if m.samples[i].length > 0 {
			lenby := m.samples[i].length * 2
			//fmt.Println("Sample length: ", lenby)
			m.samples[i].data = make([]byte, lenby)
			n, err = file.Read(m.samples[i].data)
			total += n
			if err != nil {
				return err
			}
		}
	}

	fmt.Println("total: ", total)

	return nil
}

func loadPattern(b []byte, p *pattern) {
	var sample byte
	var period, effect uint16
	var b0, b1, b2, b3 byte
	var index int

	for j := 0; j < MAX_DIVISIONS; j++ {
		//fmt.Println("Division ", j)
		for c := 0; c < MAX_CHANNELS; c++ {
			index = j * DIVISION_LEN + c * MAX_CHANNELS

			b0 = b[index]
			b1 = b[index + 1]
			b2 = b[index + 2]
			b3 = b[index + 3]

			//fmt.Printf("%d: %02x,%02x,%02x,%02x\n", index, b0, b1, b2, b3)

			sample  = b2 >> 4 | b0 & 0xF0

			period  = uint16(b1)
			period |= uint16(b0 & 0x0F) << 8

			effect  = uint16(b3)
			effect |= uint16(b2 & 0x0F) << 8

			p.divisions[j].channels[c].sample = sample
			p.divisions[j].channels[c].period = period
			p.divisions[j].channels[c].effect = effect
		}
	}
	p.initialized = true
}

func (m *module) play() {
	var p *pattern
	var d *division
	var c0, c1, c2, c3 *channel
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for ptrn := 0; ptrn < m.positions; ptrn++ {
		p = &m.patterns[m.table[ptrn]]
		fmt.Printf("|=================%02d================|\n", m.table[ptrn])
		for div := 0; div < MAX_DIVISIONS; div++ {
			d = &p.divisions[div]
			c0 = &d.channels[0]
			c1 = &d.channels[1]
			c2 = &d.channels[2]
			c3 = &d.channels[3]
			fmt.Printf("| %3s %02x | %3s %02x | %3s %02x | %3s %02x |\n",
			           notes[c0.period], c0.sample,
			           notes[c1.period], c1.sample,
			           notes[c2.period], c2.sample,
			           notes[c3.period], c3.sample)
			select {
			case _ = <-ticker.C:
			}
		}
		fmt.Printf("|========|========|========|========|\n")
	}
}
