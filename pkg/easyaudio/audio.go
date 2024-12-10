package easyaudio

import (
	"encoding/binary"
	"github.com/gordonklaus/portaudio"
	"math"
)

type AudioConfig struct {
	SampleRate float64
	Channels   int
	BufferSize int
}

func DefaultConfig() AudioConfig {
	return AudioConfig{
		SampleRate: 44100,
		Channels:   1,
		BufferSize: 1024,
	}
}

type AudioCapture struct {
	stream *portaudio.Stream
	config AudioConfig
	buffer []float32
}

func NewAudioCapture(config AudioConfig) (*AudioCapture, error) {
	err := portaudio.Initialize()
	if err != nil {
		return nil, err
	}

	capture := &AudioCapture{
		config: config,
		buffer: make([]float32, config.BufferSize),
	}

	stream, err := portaudio.OpenDefaultStream(
		config.Channels, 0,
		config.SampleRate,
		config.BufferSize,
		capture.buffer,
	)
	if err != nil {
		return nil, err
	}

	capture.stream = stream
	return capture, nil
}

func (ac *AudioCapture) Start() error {
	return ac.stream.Start()
}

func (ac *AudioCapture) Stop() error {
	return ac.stream.Stop()
}

func (ac *AudioCapture) ReadAudio() ([]byte, error) {
	err := ac.stream.Read()
	if err != nil {
		return nil, err
	}

	// Конвертируем float32 в []byte
	bytes := make([]byte, len(ac.buffer)*4)
	for i, sample := range ac.buffer {
		binary.BigEndian.PutUint32(bytes[i*4:], math.Float32bits(sample))
	}

	return bytes, nil
}

func (ac *AudioCapture) Close() error {
	err := ac.stream.Close()
	portaudio.Terminate()
	return err
}
