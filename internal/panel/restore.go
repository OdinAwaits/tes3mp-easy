package panel

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

// opsRestorer contains the import operations used by the restore workflow.
// They stay outside Ops because they are only needed by the admin restore
// handler, while existing read/deploy fakes must not grow another method.
type opsRestorer interface {
	ModsImportNamed(context.Context, string, string) (ImportResult, error)
	StateImport(context.Context, string, string) (StateImportResult, error)
}

type restorePlan struct {
	statePath string
	stateName string
	stateSHA  string
	modsPath  string
	modsName  string
	modsSHA   string
}

type restoreHTTPError struct {
	status  int
	message string
}

func (e *restoreHTTPError) Error() string { return e.message }

func restoreErrorf(status int, format string, args ...any) error {
	return &restoreHTTPError{status: status, message: fmt.Sprintf(format, args...)}
}

func (h *panelHandler) handleStateRestore(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не настроен: восстановление недоступно.", "")
		return
	}
	restorer, ok := h.ops.(opsRestorer)
	if !ok {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не умеет импортировать state и моды.", "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRestoreBodyBytes)
	upload, err := h.receiveAdminUpload(r, session.CSRF, maxRestoreUploadBytes)
	if err != nil {
		h.renderRestoreFailure(w, r, session, err)
		return
	}
	if !upload.CSRF {
		h.removeUpload(upload.Path)
		http.Error(w, "CSRF token is missing or invalid", http.StatusForbidden)
		return
	}
	if upload.Path == "" {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"В форме нет файла архива.", "file")
		return
	}
	defer h.removeUpload(upload.Path)

	plan, cleanup, err := h.prepareRestore(upload)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		h.renderRestoreFailure(w, r, session, err)
		return
	}

	outcome, err := h.restoreArchives(r.Context(), restorer, plan)
	if err != nil {
		h.logger.Printf("audit: action=state.restore archive=%s result=error code=%s",
			plan.stateName, adminErrorCode(err))
		h.renderRestoreFailure(w, r, session, err)
		return
	}
	h.logger.Printf("audit: action=state.restore archive=%s mods=%s restarted=%t result=ok",
		outcome.Archive, outcome.ModsArchive, outcome.Restarted)
	http.Redirect(w, r, "/admin/state?ok=state_restored", http.StatusSeeOther)
}

func (h *panelHandler) renderRestoreFailure(w http.ResponseWriter,
	r *http.Request, session sessionState, err error,
) {
	if isRequestTooLarge(err) {
		h.renderStateFailure(w, r, session, http.StatusRequestEntityTooLarge,
			"Архив больше 1 ГиБ.", "file")
		return
	}
	switch {
	case errors.Is(err, errMultipartUpload):
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Ожидалась multipart-форма с архивом.", "file")
		return
	case errors.Is(err, errNoUploadFile):
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"В форме нет файла архива.", "file")
		return
	}
	var restoreErr *restoreHTTPError
	if errors.As(err, &restoreErr) {
		h.renderStateFailure(w, r, session, restoreErr.status, restoreErr.message, "file")
		return
	}
	status, message, field := adminFailure(err)
	if field == "" {
		field = "file"
	}
	h.renderStateFailure(w, r, session, status, message, field)
}

func (h *panelHandler) prepareRestore(upload adminUpload) (restorePlan, func(), error) {
	kind, err := detectRestoreArchive(upload.Path)
	if err != nil {
		return restorePlan{}, nil, err
	}

	switch kind {
	case "zip":
		plan, cleanup, err := h.extractRestoreBundle(upload.Path)
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			return restorePlan{}, nil, err
		}
		return plan, cleanup, nil
	case "gzip":
		name, err := h.restoreStateName(upload.FileName)
		if err != nil {
			return restorePlan{}, nil, err
		}
		return restorePlan{statePath: upload.Path, stateName: name}, nil, nil
	default:
		return restorePlan{}, nil, restoreErrorf(http.StatusBadRequest,
			"Поддерживаются ZIP «состояние + моды» и state .tar.gz.")
	}
}

func detectRestoreArchive(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", restoreErrorf(http.StatusInternalServerError,
			"Не удалось прочитать загруженный файл.")
	}
	defer file.Close()

	var header [4]byte
	n, err := io.ReadFull(file, header[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", restoreErrorf(http.StatusBadRequest,
			"Загруженный файл пуст или повреждён.")
	}
	switch {
	case n >= 4 && header[0] == 'P' && header[1] == 'K' &&
		(header[2] == 3 && header[3] == 4 || header[2] == 5 && header[3] == 6):
		return "zip", nil
	case n >= 2 && header[0] == 0x1f && header[1] == 0x8b:
		return "gzip", nil
	default:
		return "", restoreErrorf(http.StatusBadRequest,
			"Поддерживаются ZIP «состояние + моды» и state .tar.gz.")
	}
}

func (h *panelHandler) restoreStateName(uploadName string) (string, error) {
	if archive.ValidateStateArchiveName(uploadName) == nil {
		return uploadName, nil
	}
	stateDir := filepath.Join(h.cfg.Root, "backups", "state")
	name, err := archive.NextArchiveNameInDir(stateDir, archive.KindStateManual, h.now())
	if err != nil {
		return "", restoreErrorf(http.StatusInternalServerError,
			"Не удалось выбрать имя для импортируемого состояния.")
	}
	return name, nil
}

func (h *panelHandler) extractRestoreBundle(zipPath string) (restorePlan, func(), error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
			"ZIP не читается: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
			"ZIP должен содержать ровно два файла: state и моды.")
	}

	var plan restorePlan
	for _, file := range reader.File {
		name := file.Name
		if file.FileInfo().IsDir() || strings.ContainsAny(name, `/\`) ||
			filepath.Base(name) != name {
			return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
				"ZIP должен содержать только два архива в корне.")
		}
		switch {
		case archive.ValidateStateArchiveName(name) == nil:
			if plan.stateName != "" {
				return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
					"В ZIP больше одного state-архива.")
			}
			plan.stateName = name
		case archive.ValidateModsArchiveName(name) == nil:
			if plan.modsName != "" {
				return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
					"В ZIP больше одного архива модов.")
			}
			plan.modsName = name
		default:
			return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
				"Неожиданный файл в ZIP: %s", name)
		}
	}
	if plan.stateName == "" || plan.modsName == "" {
		return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
			"ZIP должен содержать state-архив и архив модов.")
	}

	uploadsDir := filepath.Join(h.cfg.DataDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0o2770); err != nil {
		return restorePlan{}, nil, restoreErrorf(http.StatusInternalServerError,
			"Не удалось подготовить временный каталог.")
	}
	dir, err := os.MkdirTemp(uploadsDir, "restore-")
	if err != nil {
		return restorePlan{}, nil, restoreErrorf(http.StatusInternalServerError,
			"Не удалось подготовить временный каталог.")
	}
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			h.logger.Printf("panel: remove restore directory %s: %v", dir, err)
		}
	}
	// ops runs as tes3mp and moves the extracted archives out of this directory
	// with rename(2), which needs write access to the source directory, not just
	// traverse. Keep the uploads root mode (2770) so the shared group can do it;
	// set it explicitly because os.MkdirTemp only grants 0700 to the panel user
	// and custom roots may lack the setgid bit.
	if err := os.Chmod(dir, os.ModeSetgid|0o770); err != nil {
		cleanup()
		return restorePlan{}, nil, restoreErrorf(http.StatusInternalServerError,
			"Не удалось подготовить временный каталог.")
	}

	limits := archive.ZipLimits{
		CompressedSize:   maxRestoreUploadBytes,
		UncompressedSize: 2 * archive.MaxCompressedSize,
		Entries:          16,
	}
	if err := archive.ExtractZipFileWithLimits(zipPath, dir, limits); err != nil {
		cleanup()
		return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
			"ZIP не распакован: %v", err)
	}

	plan.statePath = filepath.Join(dir, plan.stateName)
	plan.modsPath = filepath.Join(dir, plan.modsName)
	for _, path := range []string{plan.statePath, plan.modsPath} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 ||
			!info.Mode().IsRegular() {
			cleanup()
			return restorePlan{}, nil, restoreErrorf(http.StatusUnprocessableEntity,
				"ZIP не содержит ожидаемый файл %s.", filepath.Base(path))
		}
		if err := os.Chmod(path, 0o640); err != nil {
			cleanup()
			return restorePlan{}, nil, restoreErrorf(http.StatusInternalServerError,
				"Не удалось выставить права на распакованный архив.")
		}
	}
	return plan, cleanup, nil
}

type restoreOutcome struct {
	Archive     string
	ModsArchive string
	Restarted   bool
}

func (h *panelHandler) restoreArchives(ctx context.Context, restorer opsRestorer,
	plan restorePlan,
) (restoreOutcome, error) {
	stateInfo, err := archive.ValidateStateLayoutFile(plan.statePath)
	if err != nil {
		return restoreOutcome{}, restoreErrorf(http.StatusUnprocessableEntity,
			"State-архив не прошёл проверку: %v", err)
	}
	stateSHA, err := panelFileSHA256(plan.statePath)
	if err != nil {
		return restoreOutcome{}, restoreErrorf(http.StatusInternalServerError,
			"Не удалось прочитать state-архив.")
	}
	plan.stateSHA = stateSHA

	if plan.modsPath != "" {
		if _, err := archive.ValidateModsFile(plan.modsPath); err != nil {
			return restoreOutcome{}, restoreErrorf(http.StatusUnprocessableEntity,
				"Архив модов не прошёл проверку: %v", err)
		}
		modsSHA, err := panelFileSHA256(plan.modsPath)
		if err != nil {
			return restoreOutcome{}, restoreErrorf(http.StatusInternalServerError,
				"Не удалось прочитать архив модов.")
		}
		plan.modsSHA = modsSHA
		if stateInfo.Current.Name != plan.modsName ||
			stateInfo.Current.SHA256 != modsSHA {
			return restoreOutcome{}, restoreErrorf(http.StatusUnprocessableEntity,
				"Архив модов не соответствует состоянию: state ожидает %s (%s), "+
					"в ZIP лежит %s (%s).",
				stateInfo.Current.Name, stateInfo.Current.SHA256,
				plan.modsName, modsSHA)
		}
		// Refuse name collisions before touching the active mods package.
		if _, err := restoreTargetMatches(
			filepath.Join(h.cfg.Root, "backups", "mods", plan.modsName),
			plan.modsSHA); err != nil {
			return restoreOutcome{}, err
		}
	}
	if _, err := restoreTargetMatches(
		filepath.Join(h.cfg.Root, "backups", "state", plan.stateName),
		plan.stateSHA); err != nil {
		return restoreOutcome{}, err
	}

	if plan.modsPath != "" {
		if err := h.ensureRestoreMods(ctx, restorer, plan); err != nil {
			return restoreOutcome{}, err
		}
		if result, err := h.ops.ModsDeploy(ctx, plan.modsName); err != nil {
			return restoreOutcome{}, err
		} else if result.Archive != plan.modsName {
			return restoreOutcome{}, restoreErrorf(http.StatusInternalServerError,
				"ops задеплоил неожиданный архив модов %q.", result.Archive)
		}
	} else if err := archive.VerifyStateMods(stateInfo,
		filepath.Join(h.cfg.Root, "backups", "mods")); err != nil {
		return restoreOutcome{}, restoreErrorf(http.StatusUnprocessableEntity,
			"Текущий пакет модов не совпадает с состоянием: %v", err)
	}

	if err := h.ensureRestoreState(ctx, restorer, plan); err != nil {
		return restoreOutcome{}, err
	}
	deployed, err := h.ops.StateDeploy(ctx, plan.stateName)
	if err != nil {
		return restoreOutcome{}, err
	}
	return restoreOutcome{
		Archive:     deployed.Archive,
		ModsArchive: deployed.ModsArchive,
		Restarted:   deployed.Restarted,
	}, nil
}

func (h *panelHandler) ensureRestoreMods(ctx context.Context, restorer opsRestorer,
	plan restorePlan,
) error {
	target := filepath.Join(h.cfg.Root, "backups", "mods", plan.modsName)
	same, err := restoreTargetMatches(target, plan.modsSHA)
	if err != nil {
		return err
	}
	if same {
		return nil
	}

	result, err := restorer.ModsImportNamed(ctx, plan.modsPath, plan.modsName)
	if err != nil {
		return err
	}
	if result.Archive != plan.modsName || result.SHA256 != plan.modsSHA {
		return restoreErrorf(http.StatusInternalServerError,
			"ops импортировал неожиданный архив модов %q.", result.Archive)
	}
	return nil
}

func (h *panelHandler) ensureRestoreState(ctx context.Context, restorer opsRestorer,
	plan restorePlan,
) error {
	target := filepath.Join(h.cfg.Root, "backups", "state", plan.stateName)
	same, err := restoreTargetMatches(target, plan.stateSHA)
	if err != nil {
		return err
	}
	if same {
		return nil
	}

	result, err := restorer.StateImport(ctx, plan.statePath, plan.stateName)
	if err != nil {
		return err
	}
	if result.Archive != plan.stateName {
		return restoreErrorf(http.StatusInternalServerError,
			"ops импортировал неожиданное состояние %q.", result.Archive)
	}
	return nil
}

// restoreTargetMatches distinguishes an absent target (import it), an identical
// target (reuse it), and a name collision with different bytes (refuse, because
// a state archive refers to the exact mods filename and cannot be rewritten).
func restoreTargetMatches(path, wantSHA string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, restoreErrorf(http.StatusInternalServerError,
			"Не удалось проверить %s.", filepath.Base(path))
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return false, restoreErrorf(http.StatusConflict,
			"%s уже существует и не является обычным файлом.", filepath.Base(path))
	}
	got, err := panelFileSHA256(path)
	if err != nil {
		return false, restoreErrorf(http.StatusInternalServerError,
			"Не удалось прочитать %s.", filepath.Base(path))
	}
	if got != wantSHA {
		return false, restoreErrorf(http.StatusConflict,
			"%s уже существует с другим содержимым; переименуйте архив в "+
				"ZIP или удалите старый файл вручную.", filepath.Base(path))
	}
	return true, nil
}

func panelFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.CopyBuffer(sum, file, make([]byte, 32*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
