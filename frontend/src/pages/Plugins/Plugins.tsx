import { useEffect, useState, type FormEvent } from 'react'
import { FileUp, Puzzle, Trash2, Upload } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { ApiError, listPlugins, unloadPlugin, uploadPlugin, type PluginInfo } from '../../lib/api'
import styles from './Plugins.module.css'

// A Scutum plugin is a .wasm module run in an in-process wazero sandbox —
// not a subprocess, not an external service. There's no manifest system:
// the real backend only ever tracks { name, path } per plugin (see
// plugin.PluginInfo in cmd/internal/plugins/registry.go) — no version, no
// author, no declared permissions/capabilities, no config schema anywhere
// in the system, and no load timestamp either. "Loaded" below is tracked
// client-side only for plugins uploaded this session — plugins that were
// already loaded when the page fetched the list show "—" rather than
// inventing a time the backend never recorded.

function Plugins() {
  const toast = useToast()
  const [plugins, setPlugins] = useState<PluginInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [loadedAt, setLoadedAt] = useState<Record<string, string>>({})
  const [unloadingName, setUnloadingName] = useState('')

  const [showLoad, setShowLoad] = useState(false)
  const [name, setName] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function loadPlugins() {
    setLoading(true)
    setApiError('')
    try {
      setPlugins(await listPlugins())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load plugins')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadPlugins()
  }, [])

  function onFileChange(f: File | undefined) {
    if (!f) {
      setFile(null)
      return
    }
    setFile(f)
    if (!name) setName(f.name.replace(/\.wasm$/i, ''))
  }

  async function unload(p: PluginInfo) {
    setUnloadingName(p.name)
    try {
      await unloadPlugin(p.name)
      await loadPlugins()
      toast(`${p.name} unloaded`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Unload failed', 'danger')
    } finally {
      setUnloadingName('')
    }
  }

  async function submitLoad(e: FormEvent) {
    e.preventDefault()
    if (!file || !name) {
      setFormError('A .wasm file and a name are required.')
      return
    }
    if (!file.name.toLowerCase().endsWith('.wasm')) {
      setFormError('Only .wasm modules can be loaded into the sandbox.')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      await uploadPlugin(name, file)
      await loadPlugins()
      setLoadedAt((prev) => ({ ...prev, [name]: new Date().toLocaleTimeString() }))
      toast(`${name} loaded`)
      setName('')
      setFile(null)
      setShowLoad(false)
    } catch (e) {
      setFormError(e instanceof ApiError ? e.message : 'Upload failed')
    } finally {
      setSaving(false)
    }
  }

  return (
    <AppShell title="Plugins">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">WASM Runtime</p>
            <h2 className={styles.reportTitle}>Plugin registry</h2>
          </div>
          <span className="stamp">{plugins.length} loaded</span>
        </div>

        <div className={styles.calloutWrap}>
          <Callout variant="info">
            Plugins run as <strong>.wasm</strong> modules inside an in-process wazero sandbox — no host filesystem or network
            access except through explicit host functions (logging, a KV store, outbound HTTP, route registration). There's no
            marketplace or declared-permissions system: every plugin gets the same fixed set of capabilities, and unloading just
            calls its <code>on_unload</code> hook and frees the sandbox.
          </Callout>
        </div>

        <Section action={<Button variant="ghost" onClick={() => setShowLoad((v) => !v)}><Upload size={14} />Load plugin</Button>}>
          {showLoad && (
            <form className={styles.loadForm} onSubmit={submitLoad}>
              <div className={styles.loadFields}>
                <div className="field">
                  <label htmlFor="pluginFile">Module</label>
                  <div className={file ? `${styles.dropzone} ${styles.dropzoneActive}` : styles.dropzone}>
                    <input
                      id="pluginFile"
                      type="file"
                      accept=".wasm"
                      className={styles.dropzoneInput}
                      onChange={(e) => onFileChange(e.target.files?.[0])}
                    />
                    <div className={styles.dropzoneLabel}>
                      <FileUp size={16} />
                      <span className={styles.dropzoneFileName}>{file?.name || 'Choose or drop a file…'}</span>
                      <span className={styles.dropzoneHint}>.wasm</span>
                    </div>
                  </div>
                </div>
                <TextField label="Name" id="pluginName" value={name} onChange={(e) => setName(e.target.value)} placeholder="slack-notifier" />
              </div>
              {formError && <p className={styles.formError}>{formError}</p>}
              <div className={styles.loadActions}>
                <Button type="submit" disabled={!file || !name || saving}>
                  {saving ? 'Loading…' : 'Load into sandbox'}
                </Button>
              </div>
            </form>
          )}

          {apiError ? (
            <Callout variant="danger">{apiError}</Callout>
          ) : loading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : plugins.length === 0 ? (
            <EmptyState icon={Puzzle} title="No plugins loaded" description="Load a .wasm module to extend Scutum without touching the core binary." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Path</th>
                  <th>Sandbox</th>
                  <th>Loaded</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {plugins.map((p) => (
                  <tr key={p.name}>
                    <td className="cell-name">{p.name}</td>
                    <td className="cell-muted">{p.path}</td>
                    <td>
                      <Badge variant="neutral">WASM</Badge>
                    </td>
                    <td className="cell-muted">{loadedAt[p.name] ?? '—'}</td>
                    <td>
                      <button
                        type="button"
                        className={styles.unloadBtn}
                        onClick={() => unload(p)}
                        disabled={unloadingName === p.name}
                        aria-label={`Unload ${p.name}`}
                      >
                        <Trash2 size={14} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>
      </div>
    </AppShell>
  )
}

export default Plugins
