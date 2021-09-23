package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

/*
Positions	Sample Value	Description
1 - 4	"RIFF"	Marks the file as a riff file. Characters are each 1 byte long.
5 - 8	File dataSize (integer)	Size of the overall file - 8 bytes, in bytes (32-bit integer). Typically, you'd fill this in after creation.
9 -12	"WAVE"	File Type Header. For our purposes, it always equals "WAVE".
13-16	"fmt "	Format chunk marker. Includes trailing null
17-20	16	Length of format data as listed above
21-22	1	Type of format (1 is PCM) - 2 byte integer
23-24	2	Number of Channels - 2 byte integer
25-28	44100	Sample Rate - 32 byte integer. Common values are 44100 (CD), 48000 (DAT). Sample Rate = Number of Samples per second, or Hertz.
29-32	176400	(Sample Rate * BitsPerSample * Channels) / 8.
33-34	4	(BitsPerSample * Channels) / 8.1 - 8 bit mono2 - 8 bit stereo/16 bit mono4 - 16 bit stereo
35-36	16	Bits per sample
37-40	"data"	"data" chunk header. Marks the beginning of the data section.
41-44	File dataSize (data)	Size of the data section.
*/

type WavHeader struct {
	Riff       [4]uint8
	FileSize   uint32
	FileType   [4]uint8
	Fmt        [4]uint8
	FmtLen     uint32
	Format     uint16
	NChannels  uint16
	SampleRate uint32
	ByteRate   uint32
	FrameSize  uint16
	SampleSize uint16
	Data       [4]uint8
	DataSize   uint32
}

func NewWavHeader() WavHeader {
	header := WavHeader{
		Riff:       [4]uint8{},
		FileSize:   0,
		FileType:   [4]uint8{},
		Fmt:        [4]uint8{},
		FmtLen:     0,
		NChannels:  0,
		SampleRate: 0,
		ByteRate:   0,
		FrameSize:  0,
		SampleSize: 0,
		Data:       [4]uint8{},
		DataSize:   0,
	}
	copy(header.Riff[:], "RIFF")
	copy(header.Fmt[:], "fmt")
	copy(header.Data[:], "data")

	return header
}

type WavCopyWriter struct {
	source string
	dest   string
	start  float32
	end    float32
	buffer [5000000]byte
}

func (writer *WavCopyWriter) write() error {
	src_header := WavHeader{}
	src_file, err := os.Open(writer.source)
	defer src_file.Close()
	if err != nil {
		return err
	}

	err = binary.Read(src_file, binary.LittleEndian, &src_header)
	if err != nil {
		return err
	}
	if string(src_header.Data[:]) != "data" {
		for string(src_header.Data[:]) != "data" {
			src_file.Seek(-3, 1)
			err = binary.Read(src_file, binary.LittleEndian, &src_header.Data)
			if err != nil {
				return err
			}
		}
		err = binary.Read(src_file, binary.LittleEndian, &src_header.DataSize)
		if err != nil {
			return err
		}
	}

	if string(src_header.FileType[:]) != "WAVE" {
		return fmt.Errorf("fmt error")
	}

	out_header := src_header
	if err != nil {
		return err
	}
	src_duration := float32(src_header.DataSize) / float32(src_header.ByteRate)
	if writer.end > src_duration {
		return fmt.Errorf("cannot cut if end is later than end of src file!")
	}
	bytes_cut_front := uint32(float32(src_header.ByteRate) * writer.start)
	bytes_cut_end := uint32(float32(src_header.ByteRate) * (src_duration - writer.end))
	cut_bytes := bytes_cut_front + bytes_cut_end
	if cut_bytes >= src_header.DataSize {
		return fmt.Errorf("cannot cut if start is later than end of src file!")
	}
	out_header.DataSize -= cut_bytes
	out_header.FileSize -= cut_bytes

	out_file, err := os.Create(writer.dest)
	defer out_file.Close()
	err = binary.Write(out_file, binary.LittleEndian, out_header)
	if err != nil {
		return err
	}

	if out_header.DataSize > 5000000 {
		return fmt.Errorf("File is too big!")
	}
	_, _ = src_file.Seek(int64(bytes_cut_front), 1)
	_, err = src_file.Read(writer.buffer[:out_header.DataSize])
	if err != nil {
		return err
	}
	_, err = out_file.Write(writer.buffer[:out_header.DataSize])
	if err != nil {
		return err
	}
	return nil
}
