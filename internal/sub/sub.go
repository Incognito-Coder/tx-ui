package sub

import (
	"context"
	"crypto/tls"
	"embed"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"x-ui/config"
	"x-ui/internal/logger"
	"x-ui/internal/util/common"
	"x-ui/internal/web/global"
	"x-ui/internal/web/middleware"
	"x-ui/internal/web/network"
	"x-ui/internal/web/service"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func init() {
	global.SetConfigLinkService(NewSubService(true, ""))
}

//go:embed html/*
var htmlFS embed.FS

type Server struct {
	httpServer *http.Server
	listener   net.Listener

	sub            *SUBController
	subService     *SubService
	settingService service.SettingService

	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		ctx:        ctx,
		cancel:     cancel,
		subService: NewSubService(true, ""),
	}
	global.SetConfigLinkService(s.subService)
	return s
}

func (s *Server) getHtmlFiles(customFolder string) ([]string, error) {
	files := make([]string, 0)
	dir, _ := os.Getwd()
	err := fs.WalkDir(os.DirFS(dir), customFolder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *Server) getHtmlTemplate(funcMap template.FuncMap, customFolder string) (*template.Template, error) {
	t := template.New("").Funcs(funcMap)
	err := fs.WalkDir(os.DirFS(customFolder), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			newT, err := t.ParseFS(os.DirFS(customFolder), path+"/*.html")
			if err != nil {
				// ignore
				return nil
			}
			t = newT
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	basePath, err := s.settingService.GetBasePath()
	if err != nil {
		return nil, err
	}
	engine.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{basePath + "panel/API/"})))

	engine.Use(func(c *gin.Context) {
		c.Set("base_path", basePath)
	})

	// set static files and template
	CustomUI, err := s.settingService.GetSubCustomUI()
	if err != nil {
		return nil, err
	}

	customFolder := "sub/html"
	if CustomUI {
		customFolder = "/etc/x-ui/html"
		// Create the custom folder if it does not exist
		if _, err := os.Stat(customFolder); os.IsNotExist(err) {
			err := os.MkdirAll(customFolder, os.ModePerm)
			if err != nil {
				logger.Error("Failed to create custom folder:", err)
				return nil, err
			}
		}
	}

	if config.IsDebug() {
		files, err := s.getHtmlFiles(customFolder)
		if err != nil {
			logger.Error("Failed to get HTML files:", err)
			return nil, err
		}
		engine.LoadHTMLFiles(files...)
	} else {
		// Check if custom folder exists and is not empty
		if _, err := os.Stat(customFolder); err == nil {
			template, err := s.getHtmlTemplate(engine.FuncMap, customFolder)
			if err != nil {
				logger.Error("Failed to get HTML template from custom folder:", err)
				return nil, err
			}
			engine.SetHTMLTemplate(template)
		} else {
			// Fallback to embedded HTML files
			tmpl := template.Must(template.New("").ParseFS(htmlFS, "html/*.html"))
			engine.SetHTMLTemplate(tmpl)
		}
	}

	subDomain, err := s.settingService.GetSubDomain()
	if err != nil {
		return nil, err
	}

	if subDomain != "" {
		engine.Use(middleware.DomainValidatorMiddleware(subDomain))
	}

	LinksPath, err := s.settingService.GetSubPath()
	if err != nil {
		return nil, err
	}

	JsonPath, err := s.settingService.GetSubJsonPath()
	if err != nil {
		return nil, err
	}

	Encrypt, err := s.settingService.GetSubEncrypt()
	if err != nil {
		return nil, err
	}

	ShowInfo, err := s.settingService.GetSubShowInfo()
	if err != nil {
		return nil, err
	}

	RemarkModel, err := s.settingService.GetRemarkModel()
	if err != nil {
		RemarkModel = "-ieo"
	}

	SubUpdates, err := s.settingService.GetSubUpdates()
	if err != nil {
		SubUpdates = "10"
	}

	SubJsonFragment, err := s.settingService.GetSubJsonFragment()
	if err != nil {
		SubJsonFragment = ""
	}

	SubJsonNoises, err := s.settingService.GetSubJsonNoises()
	if err != nil {
		SubJsonNoises = ""
	}

	SubJsonMux, err := s.settingService.GetSubJsonMux()
	if err != nil {
		SubJsonMux = ""
	}

	SubJsonRules, err := s.settingService.GetSubJsonRules()
	if err != nil {
		SubJsonRules = ""
	}

	SubTitle, err := s.settingService.GetSubTitle()
	if err != nil {
		SubTitle = ""
	}

	SubSupportUrl, err := s.settingService.GetSubSupportUrl()
	if err != nil {
		SubSupportUrl = ""
	}

	SubProfileUrl, err := s.settingService.GetSubProfileUrl()
	if err != nil {
		SubProfileUrl = ""
	}

	SubAnnounce, err := s.settingService.GetSubAnnounce()
	if err != nil {
		SubAnnounce = ""
	}

	g := engine.Group("/")

	s.sub = NewSUBController(
		g, LinksPath, JsonPath, Encrypt, ShowInfo, RemarkModel, SubUpdates,
		SubJsonFragment, SubJsonNoises, SubJsonMux, SubJsonRules, SubTitle, SubSupportUrl, SubProfileUrl, SubAnnounce)

	return engine, nil
}

func (s *Server) Start() (err error) {
	// This is an anonymous function, no function name
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	subEnable, err := s.settingService.GetSubEnable()
	if err != nil {
		return err
	}
	if !subEnable {
		return nil
	}

	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	certFile, err := s.settingService.GetSubCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetSubKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetSubListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetSubPort()
	if err != nil {
		return err
	}

	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err == nil {
			c := &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			listener = network.NewAutoHttpsListener(listener)
			listener = tls.NewListener(listener, c)
			logger.Info("Sub server running HTTPS on", listener.Addr())
		} else {
			logger.Error("Error loading certificates:", err)
			logger.Info("Sub server running HTTP on", listener.Addr())
		}
	} else {
		logger.Info("Sub server running HTTP on", listener.Addr())
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler: engine,
	}

	go func() {
		s.httpServer.Serve(listener)
	}()

	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	var err1 error
	var err2 error
	if s.httpServer != nil {
		err1 = s.httpServer.Shutdown(s.ctx)
	}
	if s.listener != nil {
		err2 = s.listener.Close()
	}
	return common.Combine(err1, err2)
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}

func (s *Server) GetConfigLinksByEmail(email string) (string, []string, error) {
	if s.subService == nil {
		s.subService = NewSubService(true, "")
	}
	return s.subService.GetConfigLinksByEmail(email)
}

