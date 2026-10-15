package dal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/alswl/chatta/pkg/common"
)

func LoadCodexBinding(path string) (common.CodexBinding, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return common.CodexBinding{}, err
	}
	var binding common.CodexBinding
	if err := json.Unmarshal(b, &binding); err != nil {
		return binding, err
	}
	if binding.BindingID == "" || binding.SessionID == "" || binding.Endpoint == "" || binding.ThreadID == "" || binding.Offset < 0 {
		return binding, errors.New("invalid Codex binding")
	}
	return binding, nil
}

func SaveCodexBinding(path string, binding common.CodexBinding) error {
	if binding.BindingID == "" || binding.SessionID == "" || binding.Endpoint == "" || binding.ThreadID == "" || binding.Offset < 0 {
		return errors.New("invalid Codex binding")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(binding, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".codex-binding-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Chmod(0600)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	return err
}
