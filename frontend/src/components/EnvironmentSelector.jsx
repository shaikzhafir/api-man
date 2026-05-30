import { useEffect, useRef, useState } from 'react'

const NAME_PATTERN = /^[a-z0-9._-]+$/

function EnvironmentSelector({
  environments,
  selected,
  onChange,
  envSource,
  activeBaseURL,
  onCreateEnvironment,
}) {
  const [open, setOpen] = useState(false)
  const [mode, setMode] = useState('list') // 'list' | 'new' | 'duplicate'
  const [name, setName] = useState('')
  const [source, setSource] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [focusIndex, setFocusIndex] = useState(-1)

  const triggerRef = useRef(null)
  const popoverRef = useRef(null)
  const inputRef = useRef(null)

  const items = environments.length > 0 ? environments : [{ name: 'dev', baseURL: '' }]
  const totalRows = items.length + 2 // envs + New + Duplicate

  const closePopover = () => {
    setOpen(false)
    setMode('list')
    setName('')
    setSource('')
    setError('')
    setFocusIndex(-1)
  }

  useEffect(() => {
    if (!open) return
    const onDocClick = (e) => {
      if (
        popoverRef.current?.contains(e.target) ||
        triggerRef.current?.contains(e.target)
      ) {
        return
      }
      closePopover()
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [open])

  useEffect(() => {
    if (open && mode === 'list' && focusIndex === -1) {
      const i = items.findIndex(e => e.name === selected)
      setFocusIndex(i >= 0 ? i : 0)
    }
  }, [open, mode, items, selected, focusIndex])

  useEffect(() => {
    if (mode !== 'list' && inputRef.current) {
      inputRef.current.focus()
      inputRef.current.select()
    }
  }, [mode])

  const openList = () => {
    setOpen(true)
    setMode('list')
    setError('')
  }

  const openNew = () => {
    setMode('new')
    setName('')
    setSource('')
    setError('')
  }

  const openDuplicate = () => {
    setMode('duplicate')
    setName(suggestDuplicateName(selected, items))
    setSource(selected)
    setError('')
  }

  const validateName = (value) => {
    if (!value) return 'name required'
    if (value.length > 64) return 'name must be 64 characters or fewer'
    if (!NAME_PATTERN.test(value)) {
      return 'name must be lowercase letters, numbers, dot, dash, or underscore'
    }
    if (items.some(e => e.name === value)) return `"${value}" already exists`
    return ''
  }

  const handleSubmit = async () => {
    const trimmed = name.trim()
    const validation = validateName(trimmed)
    if (validation) {
      setError(validation)
      return
    }
    setSaving(true)
    try {
      await onCreateEnvironment(trimmed, mode === 'duplicate' ? source : (source || ''))
      onChange(trimmed)
      closePopover()
    } catch (err) {
      setError(err.message || 'Failed to create environment')
    } finally {
      setSaving(false)
    }
  }

  const handleTriggerKey = (e) => {
    if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      openList()
    }
  }

  const handleListKey = (e) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      closePopover()
      triggerRef.current?.focus()
      return
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setFocusIndex(i => (i + 1) % totalRows)
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      setFocusIndex(i => (i - 1 + totalRows) % totalRows)
      return
    }
    if (e.key === 'Enter') {
      e.preventDefault()
      if (focusIndex < items.length) {
        onChange(items[focusIndex].name)
        closePopover()
        triggerRef.current?.focus()
      } else if (focusIndex === items.length) {
        openNew()
      } else if (focusIndex === items.length + 1) {
        openDuplicate()
      }
      return
    }
    if (e.key === 'n' || e.key === 'N') {
      e.preventDefault()
      openNew()
      return
    }
    if (e.key === 'd' || e.key === 'D') {
      e.preventDefault()
      openDuplicate()
    }
  }

  const handleFormKey = (e) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      setMode('list')
      setError('')
    } else if (e.key === 'Enter') {
      e.preventDefault()
      handleSubmit()
    }
  }

  const currentEnvName = selected || items[0]?.name || 'dev'

  return (
    <div className="environment-selector">
      <div className="env-selector-row">
        <label htmlFor="env-trigger">Env:</label>
        <div className="env-popover-wrap">
          <button
            id="env-trigger"
            ref={triggerRef}
            type="button"
            className="env-trigger"
            aria-haspopup="listbox"
            aria-expanded={open}
            onClick={() => (open ? closePopover() : openList())}
            onKeyDown={handleTriggerKey}
          >
            <span className="env-trigger-name">{currentEnvName}</span>
            <span className="env-trigger-chevron" aria-hidden="true">▾</span>
          </button>

          {open && (
            <div
              ref={popoverRef}
              className="env-popover"
              role={mode === 'list' ? 'listbox' : 'dialog'}
              onKeyDown={mode === 'list' ? handleListKey : handleFormKey}
              tabIndex={-1}
            >
              {mode === 'list' && (
                <>
                  <ul className="env-popover-list" role="presentation">
                    {items.map((env, i) => (
                      <li
                        key={env.name}
                        role="option"
                        aria-selected={env.name === selected}
                        className={`env-popover-row env-row ${i === focusIndex ? 'focused' : ''} ${env.name === selected ? 'active' : ''}`}
                        onMouseEnter={() => setFocusIndex(i)}
                        onClick={() => {
                          onChange(env.name)
                          closePopover()
                          triggerRef.current?.focus()
                        }}
                      >
                        <span className="env-popover-check" aria-hidden="true">
                          {env.name === selected ? '✓' : ''}
                        </span>
                        <span className="env-popover-name">{env.name}</span>
                        {env.baseURL && (
                          <span className="env-popover-hint">{env.baseURL}</span>
                        )}
                      </li>
                    ))}
                  </ul>
                  <div className="env-popover-divider" />
                  <div className="env-popover-actions">
                    <button
                      type="button"
                      className={`env-popover-row env-action ${focusIndex === items.length ? 'focused' : ''}`}
                      onMouseEnter={() => setFocusIndex(items.length)}
                      onClick={openNew}
                    >
                      <span className="env-popover-check" aria-hidden="true">+</span>
                      <span className="env-popover-name">New environment</span>
                      <span className="env-popover-kbd">n</span>
                    </button>
                    <button
                      type="button"
                      className={`env-popover-row env-action ${focusIndex === items.length + 1 ? 'focused' : ''}`}
                      onMouseEnter={() => setFocusIndex(items.length + 1)}
                      onClick={openDuplicate}
                    >
                      <span className="env-popover-check" aria-hidden="true">⧉</span>
                      <span className="env-popover-name">
                        Duplicate "{currentEnvName}"
                      </span>
                      <span className="env-popover-kbd">d</span>
                    </button>
                  </div>
                </>
              )}

              {mode !== 'list' && (
                <div className="env-popover-form">
                  <div className="env-form-title">
                    {mode === 'new' ? 'New environment' : `Duplicate "${selected}"`}
                  </div>

                  <label className="env-form-label" htmlFor="env-form-name">Name</label>
                  <input
                    id="env-form-name"
                    ref={inputRef}
                    type="text"
                    className="env-form-input"
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value)
                      if (error) setError('')
                    }}
                    placeholder="staging"
                    autoComplete="off"
                    spellCheck={false}
                  />

                  {mode === 'new' && (
                    <>
                      <label className="env-form-label" htmlFor="env-form-source">
                        Clone from
                      </label>
                      <select
                        id="env-form-source"
                        className="env-form-input"
                        value={source}
                        onChange={(e) => setSource(e.target.value)}
                      >
                        <option value="">Empty</option>
                        {items.map(env => (
                          <option key={env.name} value={env.name}>{env.name}</option>
                        ))}
                      </select>
                    </>
                  )}

                  <div className="env-form-path">
                    environments/{name.trim() || 'name'}.json
                  </div>

                  {error && <div className="env-form-error">{error}</div>}

                  <div className="env-form-actions">
                    <button
                      type="button"
                      className="env-form-cancel"
                      onClick={() => {
                        setMode('list')
                        setError('')
                      }}
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      className="env-form-save"
                      onClick={handleSubmit}
                      disabled={saving}
                    >
                      {saving ? 'Saving…' : 'Create'}
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {envSource && (
          <span className={`env-source-badge ${envSource}`}>
            {envSource === 'collection' ? 'collection' : 'global'}
          </span>
        )}
      </div>
      {activeBaseURL && (
        <span className="env-base-url">{activeBaseURL}</span>
      )}
    </div>
  )
}

function suggestDuplicateName(base, items) {
  if (!base) return 'copy'
  const taken = new Set(items.map(e => e.name))
  let candidate = `${base}-copy`
  let n = 2
  while (taken.has(candidate)) {
    candidate = `${base}-copy-${n}`
    n++
  }
  return candidate
}

export default EnvironmentSelector
