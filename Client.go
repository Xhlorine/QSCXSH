package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

var version string = "340"

const (
	PREPARED byte = iota
	CONNECTED
	PLAY
	PRE_DISCONNECT1
	PRE_DISCONNECT2
	DISCONNECTED
)

var atlas [8][8]byte

var chTo = make(chan byte)
var chFrom = make(chan byte)
var p Player = Player{
	"",
	"",
	Coord{0, 0},
}

func main() {

	var ip string
	var state byte = PREPARED
	var conn net.Conn
	for {
		switch {
		case state == PREPARED:
			var err error
			conn, err = DialMC(&ip)
			if err != nil {
				fmt.Println("Failed to connect, Error: ", err)
			} else {
				state = CONNECTED
			}
		case state == CONNECTED:
			fmt.Print("Your name in game: ")
			fmt.Scanln(&p.nameInGame)
			fmt.Print("Your     password: ")
			fmt.Scanln(&p.password)
			SendPacket(conn, []byte{0xff, CONNECTED}, []byte(p.PlayerIntro()))
			if Verify(conn) {
				state = PLAY
				go SendPlay(conn)
				go ResvPlay(conn)
			}
		case state == PLAY:
			Reprint()
			key, err := GetKey()
			if err != nil {
				fmt.Println("Failed to input, error: ", err)
			}
			if key == '0' {
				chFrom <- 0x00
				state = PRE_DISCONNECT1
				chTo <- 0x00
				fmt.Println("chTo get")

				fmt.Println("Packet 1 sent")
				continue
			}
			switch {
			case key == byte('a'):
				key = LEFT
			case key == byte('s'):
				key = DOWN
			case key == byte('d'):
				key = RIGHT
			case key == byte('w'):
				key = UP
			default:
				key = 0
			}
			JudgeMove(key)
			chTo <- key

		case state == PRE_DISCONNECT1:
			var buf [128]byte
			var err error
			var n int
			for buf[1] != PRE_DISCONNECT1 {
				n, err = conn.Read(buf[:])
				fmt.Println(buf[:n])
			}
			fmt.Println("Get PRE_1 Packet")
			if err != nil || n == 0 {
				fmt.Println("1.Failed to resv packet, error: ", err)
				continue
			}
			if buf[0] == 0xff && buf[1] == PRE_DISCONNECT1 {
				state = PRE_DISCONNECT2
			}
		case state == PRE_DISCONNECT2:
			var buf [16]byte
			SendPacket(conn, []byte{0xff, PRE_DISCONNECT2, p.pos.X, ' ', p.pos.Y}, []byte("\r\n\r\n"))
			n, err := conn.Read(buf[:])
			if err != nil || n == 0 {
				fmt.Println("2.Failed to resv packet, error: ", err)
				continue
			}
			if buf[0] == 0xff || buf[1] == PRE_DISCONNECT2 {
				fmt.Println("Ready to disconnect")
				conn.Close()
				state = DISCONNECTED
			}
		case state == DISCONNECTED:
			fmt.Println("Disconnected")
			return
		}
	}
}

// Use TCP to connect to specified ip:port
func DialMC(ip *string) (net.Conn, error) {
	fmt.Print("Input the server ip you'd connect:")
	fmt.Scanln(ip)
	fmt.Println("Connecting to the server. Please wait...")
	return net.Dial("tcp", *ip+":25565")
}

// Send the player's Infomation to the server, seperated by "\r\n"
func (p *Player) PlayerIntro() []byte {
	return []byte(version + " " + p.nameInGame + " " + p.password + "\r\n\r\n")
}

// Feed in specific sequence to make up a packet and send it to the server
func SendPacket(conn net.Conn, args ...[]byte) {
	var data []byte = nil
	for _, arr := range args {
		data = append(data, arr...)
	}
	conn.Write(data)
}

func Verify(conn net.Conn) bool {
	var buf [256]byte
	_, err := conn.Read(buf[:])
	if err != nil {
		fmt.Println("Failed to login, error: ", err)
		return false
	}
	if buf[1] != CONNECTED {
		fmt.Println("Wrong packet during login. ")
		return false
	}
	if buf[2] == 0x00 {
		fmt.Println("The server refused to login. ")
		return false
	}
	if buf[2] == 0x11 {
		fmt.Println("Login successfully")
		fmt.Println(buf[:])
		p.pos.X = buf[4]
		p.pos.Y = buf[6]

		//9~9+64
		for i := 0; i < 8; i++ {
			for j := 0; j < 8; j++ {
				atlas[i][j] = buf[8+8*i+j]
			}
		}
		return true
	}
	fmt.Println("Unexpected error. ")
	return false
}

const (
	NONE  byte = iota
	UP         //y--
	DOWN       //y++
	LEFT       //x--
	RIGHT      //x++
)

func ResvPlay(conn net.Conn) {
	var buf [7]byte
	var sig byte = 0x01
	for {
		select {
		case sig = <-chFrom:
		default:
			sig = 0x01
		}
		if sig == 0x00 {
			fmt.Println("ResvPlay stopped")
			return
		}

		n, err := conn.Read(buf[:])
		if err != nil || n == 0 {
			//fmt.Println("Err != nil, ", err)
			continue
		}
		if buf[0] != 0xff || buf[1] != PLAY {
			//fmt.Println("Invalid packet. ")
			continue
		}
		if buf[2] == 0x11 {
			//fmt.Println("ResvPlay running")
			continue
		}
		if buf[2] == 0x00 {
			//fmt.Println("Reprinting")
			fmt.Sscanf(string(buf[3:n]), "\r\n%v\r\n%v\r\n\r\n", &p.pos.X, &p.pos.Y)
		}
	}
}
func SendPlay(conn net.Conn) {
	var key byte
	for {
		select {
		case key = <-chTo:
		default:
			key = 0x11
		}
		if key == 0x00 {
			fmt.Println("SendPlay Stopped.")

			SendPacket(conn, []byte{0xff, PRE_DISCONNECT1}, []byte("\r\n\r\n"))
			return
		}

		SendPacket(conn, []byte{0xff, PLAY, key}, []byte("\r\n\r\n"))
		time.Sleep(50)
	}
}

// Get the key pressed
func GetKey() (byte, error) {
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadByte()
}

func JudgeMove(move byte) bool {
	if move == NONE {
		return true
	}
	if move == LEFT && !(p.pos.Y == 0 || atlas[p.pos.X][p.pos.Y-1] == 1) {
		p.pos.Y--
		fmt.Println("UP", p.pos.X, p.pos.Y)
		return true
	}
	if move == RIGHT && !(p.pos.Y == 7 || atlas[p.pos.X][p.pos.Y+1] == 1) {
		p.pos.Y++
		fmt.Println("DOWN", p.pos.X, p.pos.Y)
		return true
	}
	if move == UP && !(p.pos.X == 0 || atlas[p.pos.X-1][p.pos.Y] == 1) {
		p.pos.X--
		fmt.Println("LEFT", p.pos.X, p.pos.Y)
		return true
	}
	if move == DOWN && !(p.pos.X == 7 || atlas[p.pos.X+1][p.pos.Y] == 1) {
		p.pos.X++
		fmt.Println("RIGHT", p.pos.X, p.pos.Y)
		return true
	}
	fmt.Println("NO MOVE", p.pos.X, p.pos.Y)
	return false
}

func Reprint() {
	fmt.Println(p.pos.X, ' ', p.pos.Y)
	atlas[p.pos.X][p.pos.Y] = 2
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			if atlas[i][j] == 1 {
				fmt.Print("#")
			} else if atlas[i][j] == 2 {
				fmt.Print("@")
			} else {
				fmt.Print(" ")
			}
		}
		fmt.Print("\n")
	}
	atlas[p.pos.X][p.pos.Y] = 0
}

// type declarations
type Coord struct {
	X byte
	Y byte
}
type Player struct {
	nameInGame string
	password   string
	pos        Coord
}
