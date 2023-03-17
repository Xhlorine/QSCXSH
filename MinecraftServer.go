package main

import (
	"fmt"
	"net"
	"os"
)

const (
	PREPARED byte = iota
	CONNECTED
	PLAY
	PRE_DISCONNECT1
	PRE_DISCONNECT2
	DISCONNECTED
)

var version string = "340"
var atlas [8][8]byte = [8][8]byte{
	{1, 1, 0, 0, 1, 1, 0, 1},
	{1, 1, 0, 0, 0, 1, 0, 0},
	{1, 0, 0, 0, 1, 1, 0, 1},
	{1, 0, 0, 0, 1, 1, 0, 0},
	{0, 0, 0, 1, 1, 0, 0, 1},
	{1, 0, 1, 1, 0, 0, 1, 1},
	{1, 0, 1, 0, 0, 1, 1, 0},
	{0, 0, 0, 0, 1, 1, 1, 0},
}

func main() {
	var state byte = PREPARED
	var listen net.Listener
	var conn net.Conn
	var err error
	var buf [64]byte
	p := Player{
		//Default player info
		"Admin",
		"acltql",
		Coord{3, 0},
	}
	content, err := os.ReadFile("player.dat")
	fmt.Sscanf(string(content), "%s\n%s\n%v %v", &p.nameInGame, &p.password, &p.pos.X, &p.pos.Y)
	for {
		switch {
		// PREPARED: Ready to establish a connection
		case state == PREPARED:
			listen, err = net.Listen("tcp", ":25565")
			if err != nil {
				fmt.Println("Failed to listen, error: ", err)
			} else {
				state = CONNECTED
			}
		// CONNECTED: Connected, but haven't start game yet
		case state == CONNECTED:
			conn, err = listen.Accept()
			if err != nil {
				fmt.Println("Failed to accept, error: ", err)
			}
			for state != PLAY {
				ok, err := ProcessCONN(&p, conn)
				if err != nil {
					fmt.Println(err)
				} else if ok == true {
					fmt.Println("Login successfully")
					state = PLAY
				}
			}

		case state == PLAY:
			buf, err = GetPacket(conn)
			if err != nil {
				fmt.Println("Error while playing: ", err)
			}
			if buf[1] == PLAY {
				if !JudgeMove(&p, buf[2]) {
					SendPacket(conn, []byte{0xff, PLAY, 0x11}, []byte("\r\n\r\n"))
				} else {
					SendPacket(conn, []byte{0xff, PLAY, 0x00, ' ', p.pos.X, ' ', p.pos.Y}, []byte("\r\n\r\n"))
				}
			}
			if buf[1] == PRE_DISCONNECT1 {
				SendPacket(conn, []byte{0xff, PRE_DISCONNECT1}, []byte("\r\n\r\n"))
				fmt.Println("Apply to disconnect: Section 1.")
				state = PRE_DISCONNECT1
			}

		case state == PRE_DISCONNECT1:
			buf, err = GetPacket(conn)
			if err != nil {
				fmt.Println("Error during PRE_1: ", err)
				continue
			}
			os.WriteFile("player.dat", []byte(fmt.Sprintf("%s\n%s\n%v %v", p.nameInGame, p.password, p.pos.X, p.pos.Y)), 0666)
			SendPacket(conn, []byte{0xff, PRE_DISCONNECT2}, []byte("\r\n\r\n"))
			state = PRE_DISCONNECT2
		case state == PRE_DISCONNECT2:
			/**/ fmt.Println("Here in PRE2")
			conn.Close()
			listen.Close()
			state = DISCONNECTED
		case state == DISCONNECTED:
			fmt.Println("Disconnected")
			return
		}
	}
}

func GetPacket(conn net.Conn) ([64]byte, error) {
	var buf [64]byte
	_, err := conn.Read(buf[:])
	if err != nil {
		return buf, err
	}
	if buf[0] != 0xff {
		return buf, fmt.Errorf("Invalid packet.")
	}
	return buf, nil
}

func ProcessCONN(p *Player, conn net.Conn) (bool, error) {
	var buf [64]byte
	buf, err := GetPacket(conn)
	if err != nil {
		return false, err
	}
	if buf[1] != CONNECTED {
		return false, fmt.Errorf("Unexpected")
	}
	var name string
	var password string
	fmt.Sscanf(string(buf[5:]), " %s %s\r\n\r\n", &name, &password)
	// Test the version, name and password
	if string(buf[2:5]) != version {
		// Refuse to login
		SendPacket(conn, []byte{0xff, CONNECTED, 0x00}, []byte("\r\n\r\n"))
		return false, fmt.Errorf("Connection refused: Illegal version")
	} else if name != p.nameInGame {
		// Refuse to login
		SendPacket(conn, []byte{0xff, CONNECTED, 0x00}, []byte("\r\n\r\n"))
		return false, fmt.Errorf("Connection refused: Inexistent User")
	} else if password != p.password {
		// Refuse to login
		SendPacket(conn, []byte{0xff, CONNECTED, 0x00}, []byte("\r\n\r\n"))
		return false, fmt.Errorf("Connection refused: Wrong password")
	} else {
		// Agree to login
		SendPacket(conn, []byte{0xff, CONNECTED, 0x11}, []byte{' ', p.pos.X, ' ', p.pos.Y, ' '}, LinearAtlas(), p.PlayerIntro())
		return true, nil
	}
}

const (
	NONE  byte = iota
	UP         //y--
	DOWN       //y++
	LEFT       //x--
	RIGHT      //x++
)

func JudgeMove(p *Player, move byte) bool {
	if move == NONE {
		return true
	}
	fmt.Println(p.pos.X, p.pos.Y)
	if move == LEFT && !(p.pos.Y == 0 || atlas[p.pos.X][p.pos.Y-1] == 1) {
		p.pos.Y--
		return true
	}
	if move == RIGHT && !(p.pos.Y == 7 || atlas[p.pos.X][p.pos.Y+1] == 1) {
		p.pos.Y++
		return true
	}
	if move == UP && !(p.pos.X == 0 || atlas[p.pos.X-1][p.pos.Y] == 1) {
		p.pos.X--
		return true
	}
	if move == DOWN && !(p.pos.X == 7 || atlas[p.pos.X+1][p.pos.Y] == 1) {
		p.pos.X++
		return true
	}
	return false
}

func LinearAtlas() []byte {
	var la []byte = nil
	for _, arr := range atlas {
		la = append(la, arr[:]...)
	}
	return la
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
	fmt.Println("Sending Packet: ", data)
	conn.Write(data)
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
