package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/auth"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/cnss"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/exporter"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails boundary. Business rules remain in internal packages.
type App struct {
	ctx        context.Context
	store      *store.Store
	startupErr error
	sessionMu  sync.RWMutex
	sessionID  int64
}

type AuthState struct {
	SetupRequired bool       `json:"setupRequired"`
	Authenticated bool       `json:"authenticated"`
	User          *auth.User `json:"user"`
}

type ExportInput struct {
	Format        exporter.Format    `json:"format"`
	Company       exporter.Company   `json:"company"`
	Employees     []payroll.Employee `json:"employees"`
	Period        string             `json:"period"`
	ExecutionDate string             `json:"executionDate"`
	BatchNumber   int                `json:"batchNumber"`
}

type ExportPreview struct {
	Name          string `json:"name"`
	MediaType     string `json:"mediaType"`
	ContentBase64 string `json:"contentBase64"`
	RecordCount   int    `json:"recordCount"`
	TotalCentimes int64  `json:"totalCentimes"`
	Contract      string `json:"contract"`
}

type ExportResult struct {
	Path          string `json:"path"`
	RecordCount   int    `json:"recordCount"`
	TotalCentimes int64  `json:"totalCentimes"`
	Contract      string `json:"contract"`
}

type LargeExportInput struct {
	Format        exporter.Format  `json:"format"`
	Company       exporter.Company `json:"company"`
	Period        string           `json:"period"`
	ExecutionDate string           `json:"executionDate"`
	BatchNumber   int              `json:"batchNumber"`
}

type CNSSResult struct {
	Path           string `json:"path"`
	Imported       int64  `json:"imported"`
	ExistingCount  int64  `json:"existingCount"`
	EntrantCount   int64  `json:"entrantCount"`
	EmployeeCount  int64  `json:"employeeCount"`
	GrossCentimes  int64  `json:"grossCentimes"`
	CappedCentimes int64  `json:"cappedCentimes"`
}

func NewApp() *App {
	return &App{}
}

func (app *App) startup(ctx context.Context) {
	app.ctx = ctx
	configDir, err := os.UserConfigDir()
	if err != nil {
		app.startupErr = fmt.Errorf("dossier de configuration: %w", err)
		return
	}
	appDir := filepath.Join(configDir, "PaieMaroc")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		app.startupErr = fmt.Errorf("création du dossier de données: %w", err)
		return
	}
	app.store, app.startupErr = store.Open(filepath.Join(appDir, "paie-maroc.db"))
}

func (app *App) shutdown(context.Context) {
	if app.store != nil {
		_ = app.store.Close()
	}
}

func (app *App) GetAuthState() (AuthState, error) {
	if err := app.ready(); err != nil {
		return AuthState{}, err
	}
	hasUsers, err := app.store.HasUsers(app.ctx)
	if err != nil {
		return AuthState{}, err
	}
	if !hasUsers {
		return AuthState{SetupRequired: true}, nil
	}
	user, err := app.currentUser()
	if err != nil {
		return AuthState{}, nil
	}
	return AuthState{Authenticated: true, User: &user}, nil
}

func (app *App) CreateInitialAdmin(input auth.CreateInput) (AuthState, error) {
	if err := app.ready(); err != nil {
		return AuthState{}, err
	}
	user, err := app.store.CreateInitialAdmin(app.ctx, input)
	if err != nil {
		return AuthState{}, err
	}
	app.setSession(user.ID)
	return AuthState{Authenticated: true, User: &user}, nil
}

func (app *App) Login(username, password string) (AuthState, error) {
	if err := app.ready(); err != nil {
		return AuthState{}, err
	}
	user, err := app.store.Authenticate(app.ctx, username, password)
	if err != nil {
		return AuthState{}, err
	}
	app.setSession(user.ID)
	return AuthState{Authenticated: true, User: &user}, nil
}

func (app *App) Logout() {
	app.setSession(0)
}

func (app *App) ListUsers() ([]auth.User, error) {
	user, err := app.requireAdmin()
	if err != nil {
		return nil, err
	}
	return app.store.ListUsers(app.ctx, user.ID)
}

func (app *App) CreateUser(input auth.CreateInput) (auth.User, error) {
	user, err := app.requireAdmin()
	if err != nil {
		return auth.User{}, err
	}
	return app.store.CreateUser(app.ctx, user.ID, input)
}

func (app *App) SetUserEnabled(id int64, enabled bool) error {
	user, err := app.requireAdmin()
	if err != nil {
		return err
	}
	return app.store.SetUserEnabled(app.ctx, user.ID, id, enabled)
}

func (app *App) ResetUserPassword(id int64, password string) error {
	user, err := app.requireAdmin()
	if err != nil {
		return err
	}
	return app.store.ResetUserPassword(app.ctx, user.ID, id, password)
}

func (app *App) ListEmployees(cursor, prefix string, limit int) (store.Page, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return store.Page{}, err
	}
	return app.store.List(app.ctx, cursor, prefix, limit)
}

func (app *App) SaveEmployee(employee payroll.Employee) error {
	if _, err := app.requireAuthenticated(); err != nil {
		return err
	}
	return app.store.Upsert(app.ctx, employee)
}

func (app *App) DeleteEmployee(id string) error {
	if _, err := app.requireAuthenticated(); err != nil {
		return err
	}
	return app.store.Delete(app.ctx, id)
}

func (app *App) ImportEmployeesCSV() (store.ImportStats, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return store.ImportStats{}, err
	}
	path, err := runtime.OpenFileDialog(app.ctx, runtime.OpenDialogOptions{
		Title: "Importer les salariés depuis un CSV",
		Filters: []runtime.FileFilter{
			{DisplayName: "Fichier CSV (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return store.ImportStats{}, fmt.Errorf("ouverture du sélecteur: %w", err)
	}
	if path == "" {
		return store.ImportStats{}, errors.New("import annulé")
	}
	input, err := os.Open(path)
	if err != nil {
		return store.ImportStats{}, fmt.Errorf("ouverture CSV: %w", err)
	}
	defer input.Close()
	return app.store.ImportCSV(app.ctx, input)
}

func (app *App) ImportEmployeesCSVToPeriod(period string) (store.ImportStats, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return store.ImportStats{}, err
	}
	path, err := runtime.OpenFileDialog(app.ctx, runtime.OpenDialogOptions{
		Title: "Importer les salariés dans la paie " + period,
		Filters: []runtime.FileFilter{
			{DisplayName: "Fichier CSV (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return store.ImportStats{}, fmt.Errorf("ouverture du sélecteur: %w", err)
	}
	if path == "" {
		return store.ImportStats{}, errors.New("import annulé")
	}
	input, err := os.Open(path)
	if err != nil {
		return store.ImportStats{}, fmt.Errorf("ouverture CSV: %w", err)
	}
	defer input.Close()
	return app.store.ImportCSVForPeriod(app.ctx, period, input)
}

func (app *App) CalculatePayroll(employees []payroll.Employee) (payroll.BatchResult, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return payroll.BatchResult{}, err
	}
	result, issues := payroll.CalculateBatch(employees)
	if len(issues) > 0 {
		return payroll.BatchResult{}, fmt.Errorf(
			"%s (%s): %s",
			issues[0].EmployeeID,
			issues[0].Field,
			issues[0].Message,
		)
	}
	return result, nil
}

func (app *App) ValidatePayroll(employees []payroll.Employee) ([]payroll.ValidationIssue, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return nil, err
	}
	_, issues := payroll.CalculateBatch(employees)
	return issues, nil
}

func (app *App) PreviewExport(input ExportInput) (ExportPreview, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return ExportPreview{}, err
	}
	file, err := app.generateExport(input)
	if err != nil {
		return ExportPreview{}, err
	}
	return ExportPreview{
		Name:          file.Name,
		MediaType:     file.MediaType,
		ContentBase64: base64.StdEncoding.EncodeToString(file.Content),
		RecordCount:   file.RecordCount,
		TotalCentimes: file.TotalCentimes,
		Contract:      file.Contract,
	}, nil
}

func (app *App) SaveExport(input ExportInput) (ExportResult, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return ExportResult{}, err
	}
	file, err := app.generateExport(input)
	if err != nil {
		return ExportResult{}, err
	}

	path, err := runtime.SaveFileDialog(app.ctx, runtime.SaveDialogOptions{
		Title:           "Enregistrer le fichier de paie",
		DefaultFilename: file.Name,
	})
	if err != nil {
		return ExportResult{}, fmt.Errorf("ouverture du dialogue d'enregistrement: %w", err)
	}
	if path == "" {
		return ExportResult{}, errors.New("enregistrement annulé")
	}

	output, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return ExportResult{}, fmt.Errorf("création du fichier: %w", err)
	}
	if _, err := output.Write(file.Content); err != nil {
		_ = output.Close()
		return ExportResult{}, fmt.Errorf("écriture du fichier: %w", err)
	}
	if err := output.Close(); err != nil {
		return ExportResult{}, fmt.Errorf("finalisation du fichier: %w", err)
	}

	return ExportResult{
		Path:          path,
		RecordCount:   file.RecordCount,
		TotalCentimes: file.TotalCentimes,
		Contract:      file.Contract,
	}, nil
}

// SaveAllEmployeesExport streams the complete disk-backed payroll. SIMT500 is
// returned as a ZIP and split at its five-digit record-count boundary.
func (app *App) SaveAllEmployeesExport(input LargeExportInput) (exporter.StreamResult, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return exporter.StreamResult{}, err
	}
	executionAt, err := time.Parse("2006-01-02", input.ExecutionDate)
	if err != nil {
		return exporter.StreamResult{}, errors.New("la date d'exécution doit respecter AAAA-MM-JJ")
	}
	extension := ".csv"
	if input.Format == exporter.FormatSIMT500 {
		extension = ".zip"
	}
	path, err := runtime.SaveFileDialog(app.ctx, runtime.SaveDialogOptions{
		Title:           "Exporter toute la paie",
		DefaultFilename: "PAIE_" + strings.ToUpper(string(input.Format)) + "_" + input.Period + extension,
	})
	if err != nil {
		return exporter.StreamResult{}, fmt.Errorf("ouverture du dialogue d'enregistrement: %w", err)
	}
	if path == "" {
		return exporter.StreamResult{}, errors.New("enregistrement annulé")
	}
	output, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return exporter.StreamResult{}, fmt.Errorf("création du fichier: %w", err)
	}
	result, streamErr := exporter.Stream(
		app.ctx,
		exporter.StreamRequest{
			Format: input.Format, Company: input.Company, Period: input.Period,
			ExecutionAt: executionAt, BatchNumber: input.BatchNumber,
		},
		app.store.WalkPayroll,
		output,
	)
	closeErr := output.Close()
	if streamErr != nil {
		_ = os.Remove(path)
		return exporter.StreamResult{}, streamErr
	}
	if closeErr != nil {
		return exporter.StreamResult{}, fmt.Errorf("finalisation du fichier: %w", closeErr)
	}
	return result, nil
}

// GenerateCNSSBDS guides the user through selecting the CNSS pre-established
// file and declaration CSV. The CSV is indexed in a temporary on-disk database
// so generation remains bounded for very large employers.
func (app *App) GenerateCNSSBDS() (CNSSResult, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return CNSSResult{}, err
	}
	preEstablishedPath, err := runtime.OpenFileDialog(app.ctx, runtime.OpenDialogOptions{
		Title:   "1/3 — Choisir le fichier préétabli CNSS",
		Filters: []runtime.FileFilter{{DisplayName: "Préétabli CNSS (DS_* ou TXT)", Pattern: "DS_*;*.txt"}},
	})
	if err != nil {
		return CNSSResult{}, fmt.Errorf("sélection du préétabli: %w", err)
	}
	if preEstablishedPath == "" {
		return CNSSResult{}, errors.New("génération CNSS annulée")
	}
	declarationsPath, err := runtime.OpenFileDialog(app.ctx, runtime.OpenDialogOptions{
		Title:   "2/3 — Choisir les déclarations CNSS CSV",
		Filters: []runtime.FileFilter{{DisplayName: "Déclarations CNSS (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil {
		return CNSSResult{}, fmt.Errorf("sélection des déclarations: %w", err)
	}
	if declarationsPath == "" {
		return CNSSResult{}, errors.New("génération CNSS annulée")
	}
	outputPath, err := runtime.SaveFileDialog(app.ctx, runtime.SaveDialogOptions{
		Title:           "3/3 — Enregistrer le BDS contrôlé",
		DefaultFilename: "DS_CNSS.txt",
	})
	if err != nil {
		return CNSSResult{}, fmt.Errorf("sélection de la destination: %w", err)
	}
	if outputPath == "" {
		return CNSSResult{}, errors.New("génération CNSS annulée")
	}

	preEstablished, err := os.Open(preEstablishedPath)
	if err != nil {
		return CNSSResult{}, fmt.Errorf("ouverture du préétabli: %w", err)
	}
	defer preEstablished.Close()
	declarations, err := os.Open(declarationsPath)
	if err != nil {
		return CNSSResult{}, fmt.Errorf("ouverture des déclarations: %w", err)
	}
	defer declarations.Close()
	temporary, err := os.CreateTemp("", "paie-maroc-cnss-*.db")
	if err != nil {
		return CNSSResult{}, fmt.Errorf("création de l'index CNSS: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return CNSSResult{}, err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return CNSSResult{}, err
	}
	source, stats, err := cnss.OpenCSVSource(app.ctx, declarations, temporaryPath)
	if err != nil {
		return CNSSResult{}, err
	}
	defer source.Close()

	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return CNSSResult{}, fmt.Errorf("création du BDS: %w", err)
	}
	totals, generateErr := cnss.Generate(preEstablished, output, source)
	closeErr := output.Close()
	if generateErr != nil {
		_ = os.Remove(outputPath)
		return CNSSResult{}, generateErr
	}
	if closeErr != nil {
		return CNSSResult{}, fmt.Errorf("finalisation du BDS: %w", closeErr)
	}
	return CNSSResult{
		Path: outputPath, Imported: stats.Imported, ExistingCount: totals.ExistingCount,
		EntrantCount: totals.EntrantCount, EmployeeCount: totals.EmployeeCount,
		GrossCentimes: totals.GrossCentimes, CappedCentimes: totals.CappedCentimes,
	}, nil
}

func (app *App) generateExport(input ExportInput) (exporter.File, error) {
	if len(input.Employees) > 10_000 {
		return exporter.File{}, errors.New("utilisez l'export massif sur disque au-delà de 10 000 salariés")
	}
	batch, issues := payroll.CalculateBatch(input.Employees)
	if len(issues) > 0 {
		return exporter.File{}, fmt.Errorf(
			"corrigez %s (%s): %s",
			issues[0].EmployeeID,
			issues[0].Field,
			issues[0].Message,
		)
	}
	executionAt, err := time.Parse("2006-01-02", input.ExecutionDate)
	if err != nil {
		return exporter.File{}, errors.New("la date d'exécution doit respecter AAAA-MM-JJ")
	}
	return exporter.Generate(exporter.Request{
		Format:      input.Format,
		Company:     input.Company,
		Payroll:     batch,
		Period:      input.Period,
		ExecutionAt: executionAt,
		BatchNumber: input.BatchNumber,
	})
}

func (app *App) ready() error {
	if app.startupErr != nil {
		return app.startupErr
	}
	if app.store == nil {
		return errors.New("la base locale n'est pas prête")
	}
	return nil
}

func (app *App) ListPeriods() ([]store.Period, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return nil, err
	}
	return app.store.ListPeriods(app.ctx)
}

func (app *App) CreatePeriod(input store.CreatePeriodInput) (store.Period, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return store.Period{}, err
	}
	return app.store.CreatePeriod(app.ctx, input)
}

func (app *App) ListPeriodEmployees(period, cursor, prefix string, limit int) (store.Page, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return store.Page{}, err
	}
	return app.store.ListPeriodEmployees(app.ctx, period, cursor, prefix, limit)
}

func (app *App) SavePeriodEmployee(period string, employee payroll.Employee) error {
	if _, err := app.requireAuthenticated(); err != nil {
		return err
	}
	return app.store.SavePeriodEmployee(app.ctx, period, employee)
}

func (app *App) ClosePeriod(period string) error {
	if _, err := app.requireAuthenticated(); err != nil {
		return err
	}
	return app.store.ClosePeriod(app.ctx, period)
}

func (app *App) ReopenPeriod(period string) error {
	if _, err := app.requireAdmin(); err != nil {
		return err
	}
	return app.store.ReopenPeriod(app.ctx, period)
}

func (app *App) DeletePeriod(period string) error {
	if _, err := app.requireAdmin(); err != nil {
		return err
	}
	return app.store.DeletePeriod(app.ctx, period)
}

func (app *App) AttendanceProfiles() []attendance.Profile {
	return []attendance.Profile{attendance.MB10VLProfile()}
}

func (app *App) ListAttendanceDevices() ([]attendance.Device, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return nil, err
	}
	return app.store.ListAttendanceDevices(app.ctx)
}

func (app *App) SaveAttendanceDevice(device attendance.Device) (attendance.Device, error) {
	if _, err := app.requireAdmin(); err != nil {
		return attendance.Device{}, err
	}
	return app.store.SaveAttendanceDevice(app.ctx, device)
}

func (app *App) TestAttendanceDevice(device attendance.Device) (attendance.ProbeResult, error) {
	if _, err := app.requireAdmin(); err != nil {
		return attendance.ProbeResult{}, err
	}
	result, err := attendance.Probe(app.ctx, device)
	if err != nil {
		return attendance.ProbeResult{}, err
	}
	if device.ID != 0 {
		if err := app.store.RecordDeviceProbe(app.ctx, device.ID, result); err != nil {
			return attendance.ProbeResult{}, err
		}
	}
	return result, nil
}

func (app *App) LinkAttendanceEmployee(deviceID int64, employeeID, deviceUserID string) error {
	if _, err := app.requireAdmin(); err != nil {
		return err
	}
	return app.store.LinkAttendanceEmployee(app.ctx, deviceID, employeeID, deviceUserID)
}

func (app *App) setSession(id int64) {
	app.sessionMu.Lock()
	app.sessionID = id
	app.sessionMu.Unlock()
}

func (app *App) currentUser() (auth.User, error) {
	app.sessionMu.RLock()
	id := app.sessionID
	app.sessionMu.RUnlock()
	if id == 0 {
		return auth.User{}, errors.New("connexion requise")
	}
	user, err := app.store.GetUser(app.ctx, id)
	if err != nil || !user.Enabled {
		app.setSession(0)
		return auth.User{}, errors.New("connexion requise")
	}
	return user, nil
}

func (app *App) requireAuthenticated() (auth.User, error) {
	if err := app.ready(); err != nil {
		return auth.User{}, err
	}
	return app.currentUser()
}

func (app *App) requireAdmin() (auth.User, error) {
	user, err := app.requireAuthenticated()
	if err != nil {
		return auth.User{}, err
	}
	if user.Role != auth.RoleAdmin {
		return auth.User{}, store.ErrAdminRequired
	}
	return user, nil
}
