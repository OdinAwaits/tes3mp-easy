package panel

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
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

// restoreErrorf builds a localized HTTP error. The translator comes from the
// panel handler so the operator reads the message in the panel language.
func restoreErrorf(tr *i18n.Translator, status int, key string, args ...any) error {
	return &restoreHTTPError{status: status, message: tr.Tf(key, args...)}
}

func (h *panelHandler) handleStateRestore(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			i18n.KeyPanelAdminRestoreUnavail, "")
		return
	}
	restorer, ok := h.ops.(opsRestorer)
	if !ok {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			i18n.KeyPanelAdminModesUnsupported, "")
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
			i18n.KeyPanelAdminFileMissing, "file")
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
			i18n.KeyPanelAdminStateTooLarge, "file")
		return
	}
	switch {
	case errors.Is(err, errMultipartUpload):
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			i18n.KeyPanelAdminMultipartExpected, "file")
		return
	case errors.Is(err, errNoUploadFile):
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			i18n.KeyPanelAdminFileMissing, "file")
		return
	}
	var restoreErr *restoreHTTPError
	if errors.As(err, &restoreErr) {
		h.renderStateFailure(w, r, session, restoreErr.status, restoreErr.message, "file")
		return
	}
	status, message, field := h.adminFailure(err)
	if field == "" {
		field = "file"
	}
	h.renderStateFailure(w, r, session, status, message, field)
}

func (h *panelHandler) prepareRestore(upload adminUpload) (restorePlan, func(), error) {
	tr := h.tr()
	kind, err := detectRestoreArchive(h.tr(), upload.Path)
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
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusBadRequest,
			i18n.KeyPanelRestoreUnsupported)
	}
}

func detectRestoreArchive(tr *i18n.Translator, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreReadFailed)
	}
	defer file.Close()

	var header [4]byte
	n, err := io.ReadFull(file, header[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", restoreErrorf(tr, http.StatusBadRequest,
			i18n.KeyPanelRestoreEmpty)
	}
	switch {
	case n >= 4 && header[0] == 'P' && header[1] == 'K' &&
		(header[2] == 3 && header[3] == 4 || header[2] == 5 && header[3] == 6):
		return "zip", nil
	case n >= 2 && header[0] == 0x1f && header[1] == 0x8b:
		return "gzip", nil
	default:
		return "", restoreErrorf(tr, http.StatusBadRequest,
			i18n.KeyPanelRestoreUnsupported)
	}
}

func (h *panelHandler) restoreStateName(uploadName string) (string, error) {
	tr := h.tr()
	if archive.ValidateStateArchiveName(uploadName) == nil {
		return uploadName, nil
	}
	stateDir := filepath.Join(h.cfg.Root, "backups", "state")
	name, err := archive.NextArchiveNameInDir(stateDir, archive.KindStateManual, h.now())
	if err != nil {
		return "", restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreNameFailed)
	}
	return name, nil
}

func (h *panelHandler) extractRestoreBundle(zipPath string) (restorePlan, func(), error) {
	tr := h.tr()
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
			i18n.KeyPanelRestoreZipUnreadable, err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
			i18n.KeyPanelRestoreZipTwoFiles)
	}

	var plan restorePlan
	for _, file := range reader.File {
		name := file.Name
		if file.FileInfo().IsDir() || strings.ContainsAny(name, `/\`) ||
			filepath.Base(name) != name {
			return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
				i18n.KeyPanelRestoreZipRootOnly)
		}
		switch {
		case archive.ValidateStateArchiveName(name) == nil:
			if plan.stateName != "" {
				return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
					i18n.KeyPanelRestoreZipTwoStates)
			}
			plan.stateName = name
		case archive.ValidateModsArchiveName(name) == nil:
			if plan.modsName != "" {
				return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
					i18n.KeyPanelRestoreZipTwoMods)
			}
			plan.modsName = name
		default:
			return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
				i18n.KeyPanelRestoreZipUnexpected, name)
		}
	}
	if plan.stateName == "" || plan.modsName == "" {
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
			i18n.KeyPanelRestoreZipStateAndMods)
	}

	uploadsDir := filepath.Join(h.cfg.DataDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0o2770); err != nil {
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreTempDir)
	}
	dir, err := os.MkdirTemp(uploadsDir, "restore-")
	if err != nil {
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreTempDir)
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
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreTempDir)
	}

	limits := archive.ZipLimits{
		CompressedSize:   maxRestoreUploadBytes,
		UncompressedSize: 2 * archive.MaxCompressedSize,
		Entries:          16,
	}
	if err := archive.ExtractZipFileWithLimits(zipPath, dir, limits); err != nil {
		cleanup()
		return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
			i18n.KeyPanelRestoreUnzipFailed, err)
	}

	plan.statePath = filepath.Join(dir, plan.stateName)
	plan.modsPath = filepath.Join(dir, plan.modsName)
	for _, path := range []string{plan.statePath, plan.modsPath} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 ||
			!info.Mode().IsRegular() {
			cleanup()
			return restorePlan{}, nil, restoreErrorf(tr, http.StatusUnprocessableEntity,
				i18n.KeyPanelRestoreZipMissing, filepath.Base(path))
		}
		if err := os.Chmod(path, 0o640); err != nil {
			cleanup()
			return restorePlan{}, nil, restoreErrorf(tr, http.StatusInternalServerError,
				i18n.KeyPanelRestorePermissions)
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
	tr := h.tr()
	stateInfo, err := archive.ValidateStateLayoutFile(plan.statePath)
	if err != nil {
		return restoreOutcome{}, restoreErrorf(tr, http.StatusUnprocessableEntity,
			i18n.KeyPanelRestoreStateCheck, err)
	}
	stateSHA, err := panelFileSHA256(plan.statePath)
	if err != nil {
		return restoreOutcome{}, restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreStateRead)
	}
	plan.stateSHA = stateSHA

	if plan.modsPath != "" {
		if _, err := archive.ValidateModsFile(plan.modsPath); err != nil {
			return restoreOutcome{}, restoreErrorf(tr, http.StatusUnprocessableEntity,
				i18n.KeyPanelRestoreModsCheck, err)
		}
		modsSHA, err := panelFileSHA256(plan.modsPath)
		if err != nil {
			return restoreOutcome{}, restoreErrorf(tr, http.StatusInternalServerError,
				i18n.KeyPanelRestoreModsRead)
		}
		plan.modsSHA = modsSHA
		if stateInfo.Current.Name != plan.modsName ||
			stateInfo.Current.SHA256 != modsSHA {
			return restoreOutcome{}, restoreErrorf(tr, http.StatusUnprocessableEntity,
				i18n.KeyPanelRestoreModsMismatch,
				stateInfo.Current.Name, stateInfo.Current.SHA256,
				plan.modsName, modsSHA)
		}
		// Refuse name collisions before touching the active mods package.
		if _, err := restoreTargetMatches(h.tr(),
			filepath.Join(h.cfg.Root, "backups", "mods", plan.modsName),
			plan.modsSHA); err != nil {
			return restoreOutcome{}, err
		}
	}
	if _, err := restoreTargetMatches(h.tr(),
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
			return restoreOutcome{}, restoreErrorf(tr, http.StatusInternalServerError,
				i18n.KeyPanelRestoreDeployUnexpected, result.Archive)
		}
	} else if err := archive.VerifyStateMods(stateInfo,
		filepath.Join(h.cfg.Root, "backups", "mods")); err != nil {
		return restoreOutcome{}, restoreErrorf(tr, http.StatusUnprocessableEntity,
			i18n.KeyPanelRestoreModsMismatchLive, err)
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
	tr := h.tr()
	target := filepath.Join(h.cfg.Root, "backups", "mods", plan.modsName)
	same, err := restoreTargetMatches(h.tr(), target, plan.modsSHA)
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
		return restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreImportMods, result.Archive)
	}
	return nil
}

func (h *panelHandler) ensureRestoreState(ctx context.Context, restorer opsRestorer,
	plan restorePlan,
) error {
	tr := h.tr()
	target := filepath.Join(h.cfg.Root, "backups", "state", plan.stateName)
	same, err := restoreTargetMatches(h.tr(), target, plan.stateSHA)
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
		return restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreImportState, result.Archive)
	}
	return nil
}

// restoreTargetMatches distinguishes an absent target (import it), an identical
// target (reuse it), and a name collision with different bytes (refuse, because
// a state archive refers to the exact mods filename and cannot be rewritten).
func restoreTargetMatches(tr *i18n.Translator, path, wantSHA string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreFileCheck, filepath.Base(path))
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return false, restoreErrorf(tr, http.StatusConflict,
			i18n.KeyPanelRestoreNotRegular, filepath.Base(path))
	}
	got, err := panelFileSHA256(path)
	if err != nil {
		return false, restoreErrorf(tr, http.StatusInternalServerError,
			i18n.KeyPanelRestoreFileRead, filepath.Base(path))
	}
	if got != wantSHA {
		return false, restoreErrorf(tr, http.StatusConflict,
			i18n.KeyPanelRestoreExistsDiffers, filepath.Base(path))
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
