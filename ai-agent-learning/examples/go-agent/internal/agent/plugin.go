package agent

import "fmt"

// Plugin is the Go-friendly explicit-registration alternative to runtime
// loading of shared objects.
type Plugin interface {
	Name() string
	Register(*Registry) error
	Close() error
}

type PluginManager struct {
	plugins []Plugin
}

func (m *PluginManager) Load(plugin Plugin, registry *Registry) error {
	if plugin == nil {
		return fmt.Errorf("plugin cannot be nil")
	}
	if err := plugin.Register(registry); err != nil {
		return fmt.Errorf("register plugin %q: %w", plugin.Name(), err)
	}
	m.plugins = append(m.plugins, plugin)
	return nil
}

func (m *PluginManager) Close() error {
	for index := len(m.plugins) - 1; index >= 0; index-- {
		if err := m.plugins[index].Close(); err != nil {
			return fmt.Errorf("close plugin %q: %w", m.plugins[index].Name(), err)
		}
	}
	return nil
}
