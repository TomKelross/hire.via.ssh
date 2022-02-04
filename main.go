package main

// An example Bubble Tea server. This will put an ssh session into alt screen
// and continually print up to date terminal information.

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/bcrypt"
)

const host = "0.0.0.0"
const port = 23234

// PublicKeyHandler returns whether or not the given public key may access the
// repo.
func handler(ctx ssh.Context, pk ssh.PublicKey) bool {
	log.Print("its okay!")
	return true
}

// type PasswordHandler func(ctx Context, password string) bool
func password_handler(ctx ssh.Context, password string) bool {
	return true
}

func main() {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		// ssh.PublicKeyAuth(Handler),
		wish.WithPublicKeyAuth(handler),
		// wish.WithPasswordAuth(password_handler),
		wish.WithHostKeyPath(".ssh/term_info_ed25519"),
		wish.WithMiddleware(
			bm.Middleware(teaHandler),
			lm.Middleware(),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Starting SSH server on %s:%d", host, port)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	<-done
	log.Println("Stopping SSH server")
	ctx := context.Background()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalln(err)
	}
}

// You can wire any Bubble Tea model up to the middleware with a function that
// handles the incoming ssh.Session. Here we just grab the terminal info and
// pass it to the new model. You can also return tea.ProgramOptions (such as
// teaw.WithAltScreen) on a session by session basis
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, active := s.Pty()
	if !active {
		fmt.Println("no active terminal, skipping")
		return nil, nil
	}

	ti := textinput.New()
	ti.Placeholder = "Codeword"
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 20

	password := os.Getenv("PASSWORD")
	if password == "" {
		log.Fatalln("No PASSWORD environment variable set")
	}

	m := model{
		// This is fine to be in source control, for now, but it's not a good thing to be in source control.
		// We'll pull this out to an env var later.
		password:       password,
		term:           pty.Term,
		width:          pty.Window.Width,
		height:         pty.Window.Height,
		authenticated:  false,
		attempted_auth: false,
		textInput:      ti,
	}
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	password       string
	term           string
	width          int
	height         int
	authenticated  bool
	attempted_auth bool
	textInput      textinput.Model
}

func (m model) Init() tea.Cmd {

	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "enter":
			if !m.authenticated {
				m.attempted_auth = true

				hashed, _ := bcrypt.GenerateFromPassword([]byte(m.password), bcrypt.DefaultCost)
				// Is it safe to throw that error away?
				err := bcrypt.CompareHashAndPassword(hashed, []byte(m.textInput.Value()))
				if err == nil {
					m.authenticated = true
					// TODO: Worth having an error message if we cant hash??
				}
				log.Print(string(hashed))
				log.Print(m.textInput.Value())
			}
		}
	}

	m.textInput, _ = m.textInput.Update(msg)

	return m, nil
}

var incorrect_password_style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))

func (m model) View() string {

	if !m.authenticated {
		banner := `# Hire Tom

		Hi! I'm Tom Kelross, a (something?) engineer,  
		and this is how you access my CV!

		Just type in the password I gave you below.

		`
		banner_rendered, err := glamour.Render(banner, "dark")
		if err != nil {
			return "Could not render banner, something is seriously broken behind the scenes. Please email me instead :)"
		}

		password_feedback_message := ""
		if m.attempted_auth {
			password_feedback_message = incorrect_password_style.Render("Incorrect password")
		}

		result := fmt.Sprintf(
			banner_rendered+"%s\n%s\n%s",
			m.textInput.View(),
			"(Press enter to continue)",
			password_feedback_message,
		)
		return result

	}

	in := `# Hire Tom

	You are early! Check back later for my CV.

	To anyone snooping around in the commit history you won't be able to find my CV here ;).
	When I add it in I think i'll put it in through a fly secret, for this mvp only really needs
	to be one CV and one password. But seeing as you are here, you should definetly send me an email at
	tom@mygithubusername.com
	`

	out, err := glamour.Render(in, "dark")

	if err != nil {
		return "Oh no! Something went wrong and I couldn't render my CV. Please try again later or email me :)"
	}
	return out
}
