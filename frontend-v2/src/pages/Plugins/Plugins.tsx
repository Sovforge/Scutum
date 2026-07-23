import { useState, type FormEvent } from 'react'
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
import styles from './Plugins.module.css'

// A Scutum plugin is a .wasm module run in an in-process wazero sandbox —
// not a subprocess, not an external service. There's no manifest system:
// the real backend only ever tracks { name, path } per plugin (see
// plugin.PluginInfo in cmd/internal/plugins/registry.go) — no version, no
// author, no declared permissions/capabilities, no config schema anywhere
// in the system. A plugin's actual capabilities are just whatever host
// functions the runtime exposes (log, http_request, kv_get/set,
// register_route) — the same for every plugin, not something granted or
// shown per-plugin. `loadedAt` isn't part of the real API response either
// (the old Nuxt frontend's own PluginRecord.loadedAt field is dead — the
// backend never returns it); shown here as a locally-tracked "since this
// mock session" timestamp instead of inventing a richer backend than
// actually exists. Not wired to a live backend yet.
type Plugin = {
  name: string
  path: string
  loadedAt: string
}

const INITIAL_PLUGINS: Plugin[] = [
  { name: 'slack-notifier', path: '/var/lib/scutum/plugins/slack-notifier.wasm', loadedAt: '2026-07-02 09:14:00' },
  { name: 'webhook-router', path: '/var/lib/scutum/plugins/webhook-router.wasm', loadedAt: '2026-07-05 11:40:12' },
]

function Plugins() {
  const toast = useToast()
  const [plugins, setPlugins] = useState(INITIAL_PLUGINS)
  const [showLoad, setShowLoad] = useState(false)
  const [name, setName] = useState('')
  const [file, setFile] = useState('')
  const [formError, setFormError] = useState('')

  function onFileChange(f: File | undefined) {
    if (!f) {
      setFile('')
      return
    }
    setFile(f.name)
    if (!name) setName(f.name.replace(/\.wasm$/i, ''))
  }

  function unload(p: Plugin) {
    setPlugins((prev) => prev.filter((x) => x.name !== p.name))
    toast(`${p.name} unloaded`, 'danger')
  }

  function submitLoad(e: FormEvent) {
    e.preventDefault()
    if (!file || !name) {
      setFormError('A .wasm file and a name are required.')
      return
    }
    if (!file.toLowerCase().endsWith('.wasm')) {
      setFormError('Only .wasm modules can be loaded into the sandbox.')
      return
    }
    setFormError('')
    setPlugins((prev) => [...prev, { name, path: `/var/lib/scutum/plugins/${file}`, loadedAt: 'just now' }])
    toast(`${name} loaded`)
    setName('')
    setFile('')
    setShowLoad(false)
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
                      <span className={styles.dropzoneFileName}>{file || 'Choose or drop a file…'}</span>
                      <span className={styles.dropzoneHint}>.wasm</span>
                    </div>
                  </div>
                </div>
                <TextField label="Name" id="pluginName" value={name} onChange={(e) => setName(e.target.value)} placeholder="slack-notifier" />
              </div>
              {formError && <p className={styles.formError}>{formError}</p>}
              <div className={styles.loadActions}>
                <Button type="submit" disabled={!file || !name}>
                  Load into sandbox
                </Button>
              </div>
            </form>
          )}

          {plugins.length === 0 ? (
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
                    <td className="cell-muted">{p.loadedAt}</td>
                    <td>
                      <button type="button" className={styles.unloadBtn} onClick={() => unload(p)} aria-label={`Unload ${p.name}`}>
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
