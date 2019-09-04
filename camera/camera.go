package camera

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"sync"
	"syscall"
	"unsafe"
)

const (
	V4l2BufTypeVideoCapture = 1
	V4l2PixFmtJpeg          = 0x4745504a
	V4l2FieldNone           = 1
	V4l2MemoryMmap          = 1
	VidiocSFmt              = 0xc0cc5605
	VidiocReqbufs           = 0xc0145608
	VidiocQuerybuf          = 0xc0445609
	VidiocStreamon          = 0x40045612
	VidiocStreamoff         = 0x40045613
	VidiocQbuf              = 0xc044560f
	VidiocDqbuf             = 0xc0445611
)

type v4l2PixFormat struct {
	typ          uint32
	width        uint32
	height       uint32
	pixelformat  uint32
	field        uint32
	bytesperline uint32
	sizeimage    uint32
	colorspace   uint32
	priv         uint32
}

type v4l2Requestbuffers struct {
	count    uint32
	typ      uint32
	memory   uint32
	reserved [2]uint32
}

type v4l2Timecode struct {
	typ      uint32
	flags    uint32
	frames   uint8
	seconds  uint8
	minutes  uint8
	hours    uint8
	userbits [4]uint8
}

type timeval struct {
	tv_sec  uint32
	tv_usec uint32
}

type v4l2Buffer struct {
	index     uint32
	typ       uint32
	bytesused uint32
	flags     uint32
	field     uint32
	timestamp timeval
	timecode  v4l2Timecode
	sequence  uint32
	memory    uint32
	offset    uint32
	length    uint32
	reserved2 uint32
	reserved  uint32
}

type Camera struct {
	width    uint32
	height   uint32
	device   string
	fd       int
	mutex    sync.RWMutex
	imageLen uint32
	imageBuf []byte
	data     []byte
	buf      v4l2Buffer
	isReady  bool
	ready    chan bool
}

func NewCamera(device string, with uint32, height uint32) *Camera {
	c := Camera{
		width:   with,
		height:  height,
		device:  device,
		isReady: false,
		ready:   make(chan bool),
	}

	return &c
}

func (c *Camera) Start() error {
	dev, err := unix.Open(c.device, unix.O_RDWR|unix.O_NONBLOCK, 0666)
	if err != nil {
		return err
	}
	c.fd = dev
	log.Println("opened fd:", c.fd)

	if err := c.start(); err != nil {
		return err
	}

	go c.framePump()

	<-c.ready

	return nil
}

func (c *Camera) Stop() {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(c.fd),
		uintptr(VidiocStreamoff),
		uintptr(unsafe.Pointer(&c.buf.typ)),
	)
	if errno != 0 {
		log.Printf("fail to stop stream: %d\n", errno)
	}

	unix.Munmap(c.data)

	unix.Close(c.fd)
}

func (c *Camera) start() error {
	// Set format
	format := v4l2PixFormat{
		typ:         V4l2BufTypeVideoCapture,
		width:       c.width,
		height:      c.height,
		pixelformat: V4l2PixFmtJpeg,
		field:       V4l2FieldNone,
	}

	_, _, errno := unix.Syscall(
		syscall.SYS_IOCTL,
		uintptr(c.fd),
		uintptr(VidiocSFmt),
		uintptr(unsafe.Pointer(&format)),
	)
	if errno != 0 {
		return errors.New(fmt.Sprintf("fail to set format:%d", errno))
	}
	log.Println("set format successfully")

	// Request buffer
	req := v4l2Requestbuffers{
		count:  1,
		typ:    V4l2BufTypeVideoCapture,
		memory: V4l2MemoryMmap,
	}
	log.Println("requesting buffer...")
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(c.fd),
		uintptr(VidiocReqbufs),
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		return errors.New(fmt.Sprintf("fail to request buffer:%d", errno))
	}

	log.Println("requested buffer successfully")

	// Query buffer parameters (namely memory offset and length)
	c.buf = v4l2Buffer{
		typ:    V4l2BufTypeVideoCapture,
		memory: V4l2MemoryMmap,
		index:  0,
	}
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(c.fd),
		uintptr(VidiocQuerybuf),
		uintptr(unsafe.Pointer(&c.buf)),
	)

	if errno != 0 {
		return errors.New(fmt.Sprintf("fail to query buffer:%d", errno))
	}

	log.Println("query buffer successfully")

	// Map memory
	c.data, _ = unix.Mmap(
		c.fd,
		int64(c.buf.offset),
		int(c.buf.length),
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)

	c.imageBuf = make([]byte, len(c.data))

	qbuf := v4l2Buffer{
		typ:    V4l2BufTypeVideoCapture,
		memory: V4l2MemoryMmap,
		index:  0,
	}
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(c.fd),
		uintptr(VidiocQbuf),
		uintptr(unsafe.Pointer(&qbuf)),
	)
	if errno != 0 {
		return errors.New(fmt.Sprintf("fail to enque initial buffer:%d", errno))
	}

	log.Println("map memory successfully")

	// Start stream
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(c.fd),
		uintptr(VidiocStreamon),
		uintptr(unsafe.Pointer(&c.buf.typ)),
	)
	if errno != 0 {
		return errors.New(fmt.Sprintf("fail to start stream:%d", errno))
	}

	return nil
}

func (c *Camera) framePump() error {
	// File descriptor set
	fds := unix.FdSet{}

	// Set bit in set corresponding to file descriptor
	fds.Bits[c.fd>>8] |= 1 << (uint(c.fd) & 63)

	for {
		// Wait for frame
		_, err := unix.Select(c.fd+1, &fds, nil, nil, nil)
		if err != nil {
			return err
		}

		if !c.isReady {
			c.isReady = true
			c.ready <- true
		}

		// Dequeue buffer
		qbuf := v4l2Buffer{
			typ:
			V4l2BufTypeVideoCapture,
			memory: V4l2MemoryMmap,
			index:  0,
		}
		_, _, errno := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(c.fd),
			uintptr(VidiocDqbuf),
			uintptr(unsafe.Pointer(&qbuf)),
		)
		if errno != 0 {
			//			mutex.Unlock()
			return errno
		}

		// Lock for writing
		c.mutex.Lock()

		// Save buffer size
		c.imageLen = qbuf.bytesused

		copy(c.imageBuf, c.data)

		// Unlock for readers
		c.mutex.Unlock()

		// Enqueue buffer
		_, _, errno = syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(c.fd),
			uintptr(VidiocQbuf),
			uintptr(unsafe.Pointer(&qbuf)),
		)
		if errno != 0 {
			return errno
		}
	}
}

func (c *Camera) GetFrame() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.imageBuf[:c.imageLen]
}
