import { useState, type FormEvent } from 'react'
import { KeyRound } from 'lucide-react'
import Button from '../../../components/ui/Button/Button'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { ApiError, erkGenerateShares, erkReissueShares } from '../../../lib/api'
import styles from '../Settings.module.css'

// Real endpoints: POST /recovery/generate-shares {n_shares,threshold} ->
// {shares}, POST /recovery/reissue-shares {shares,n_shares,threshold} ->
// {new_shares}. This is Shamir's Secret Sharing over the cluster's master
// encryption key — distinct from the personal MFA recovery codes on the
// Account page. Only the "local" KMS provider supports it; there's no
// dedicated "what provider is this" endpoint, so — same as the old app —
// this just attempts the real call and surfaces whatever error comes back
// (a clear 400 "only supported with the local KMS provider" when it isn't)
// rather than pre-guessing the provider client-side.

function RecoveryTab() {
  const toast = useToast()

  // ── Generate ─────────────────────────────────────────────────────
  const [nShares, setNShares] = useState(5)
  const [threshold, setThreshold] = useState(3)
  const [generatedShares, setGeneratedShares] = useState<string[] | null>(null)
  const [genError, setGenError] = useState('')
  const [generating, setGenerating] = useState(false)

  function clampThreshold(n: number) {
    setNShares(n)
    setThreshold((t) => Math.min(t, n))
  }

  async function submitGenerate(e: FormEvent) {
    e.preventDefault()
    setGenerating(true)
    setGenError('')
    try {
      setGeneratedShares(await erkGenerateShares(nShares, threshold))
      toast('Recovery shares generated')
    } catch (err) {
      setGenError(err instanceof ApiError ? err.message : 'Failed to generate shares')
    } finally {
      setGenerating(false)
    }
  }

  // ── Reissue ──────────────────────────────────────────────────────
  const [reissueInput, setReissueInput] = useState('')
  const [reissueN, setReissueN] = useState(5)
  const [reissueThreshold, setReissueThreshold] = useState(3)
  const [reissuedShares, setReissuedShares] = useState<string[] | null>(null)
  const [reissueError, setReissueError] = useState('')
  const [reissuing, setReissuing] = useState(false)

  const suppliedShares = reissueInput
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)

  function clampReissueThreshold(n: number) {
    setReissueN(n)
    setReissueThreshold((t) => Math.min(t, n))
  }

  async function submitReissue(e: FormEvent) {
    e.preventDefault()
    if (suppliedShares.length < 2) return
    setReissuing(true)
    setReissueError('')
    try {
      const shares = await erkReissueShares(suppliedShares, reissueN, reissueThreshold)
      setReissuedShares(shares)
      setReissueInput('')
      toast('Recovery shares reissued — previous shares are now invalid', 'danger')
    } catch (err) {
      setReissueError(err instanceof ApiError ? err.message : 'Failed to reissue shares')
    } finally {
      setReissuing(false)
    }
  }

  return (
    <>
      <Section title="Generate recovery shares">
        <div className={styles.recoveryBody}>
          <p className={styles.recoveryIntro}>
            Scutum uses Shamir's Secret Sharing to split the cluster's encryption master key into recovery shares. If you ever
            lose access to the server, the key can be reconstructed from the required number of shares. Only supported with
            the local KMS provider — cloud KMS backends manage the key externally.
          </p>

          <form onSubmit={submitGenerate}>
            <div className="field">
              <label htmlFor="nShares">Total shares ({nShares})</label>
              <input
                id="nShares"
                type="range"
                min={3}
                max={10}
                value={nShares}
                onChange={(e) => clampThreshold(Number(e.target.value))}
                className={styles.rangeInput}
              />
              <div className={styles.rangeLabels}>
                <span>3</span>
                <span>10</span>
              </div>
            </div>
            <div className="field">
              <label htmlFor="threshold">
                Shares required to recover ({threshold} of {nShares})
              </label>
              <input
                id="threshold"
                type="range"
                min={2}
                max={nShares}
                value={threshold}
                onChange={(e) => setThreshold(Number(e.target.value))}
                className={styles.rangeInput}
              />
              <div className={styles.rangeLabels}>
                <span>2</span>
                <span>{nShares}</span>
              </div>
            </div>
            <div className={styles.recoveryPreview}>
              <KeyRound size={14} />
              <span>
                <strong>
                  {threshold} of {nShares}
                </strong>{' '}
                shares needed to recover access. You can lose up to <strong>{nShares - threshold}</strong> share
                {nShares - threshold !== 1 ? 's' : ''} and still recover.
              </span>
            </div>
            {genError && (
              <div className={styles.calloutWrap}>
                <Callout variant="danger">{genError}</Callout>
              </div>
            )}
            <div className={styles.formActions}>
              <Button type="submit" disabled={generating}>
                {generating ? 'Generating…' : 'Generate shares'}
              </Button>
            </div>
          </form>

          {generatedShares && (
            <>
              <Callout variant="danger">
                These shares are shown <strong>once</strong> and won't be shown again — distribute them to trusted people and
                store offline.
              </Callout>
              <div className={styles.codeGrid}>
                {generatedShares.map((s) => (
                  <span className={styles.codeChip} key={s}>
                    {s}
                  </span>
                ))}
              </div>
            </>
          )}
        </div>
      </Section>

      <Section title="Reissue shares">
        <div className={styles.recoveryBody}>
          <p className={styles.recoveryIntro}>
            Rotate the existing share set — paste at least two current shares to prove access, choose a new split, and every
            previously issued share is invalidated.
          </p>
          <form onSubmit={submitReissue}>
            <div className="field">
              <label htmlFor="existingShares">Existing shares (one per line, at least 2)</label>
              <textarea
                id="existingShares"
                className={styles.textarea}
                rows={4}
                value={reissueInput}
                onChange={(e) => setReissueInput(e.target.value)}
                placeholder={'1-a3f9c2e1d4b8f0a7c6e5d4b3a2f1e0d9\n2-8b0c3f1a2d9e7c6b5a4f3e2d1c0b9a8f'}
              />
            </div>
            <div className="field">
              <label htmlFor="reissueN">New total shares ({reissueN})</label>
              <input
                id="reissueN"
                type="range"
                min={3}
                max={10}
                value={reissueN}
                onChange={(e) => clampReissueThreshold(Number(e.target.value))}
                className={styles.rangeInput}
              />
              <div className={styles.rangeLabels}>
                <span>3</span>
                <span>10</span>
              </div>
            </div>
            <div className="field">
              <label htmlFor="reissueThreshold">
                New threshold ({reissueThreshold} of {reissueN})
              </label>
              <input
                id="reissueThreshold"
                type="range"
                min={2}
                max={reissueN}
                value={reissueThreshold}
                onChange={(e) => setReissueThreshold(Number(e.target.value))}
                className={styles.rangeInput}
              />
              <div className={styles.rangeLabels}>
                <span>2</span>
                <span>{reissueN}</span>
              </div>
            </div>
            {reissueError && (
              <div className={styles.calloutWrap}>
                <Callout variant="danger">{reissueError}</Callout>
              </div>
            )}
            <div className={styles.formActions}>
              <Button type="submit" disabled={suppliedShares.length < 2 || reissuing}>
                {reissuing ? 'Reissuing…' : 'Reissue shares'}
              </Button>
            </div>
          </form>

          {reissuedShares && (
            <>
              <Callout variant="danger">
                New shares shown <strong>once</strong> below — the shares you just supplied no longer work.
              </Callout>
              <div className={styles.codeGrid}>
                {reissuedShares.map((s) => (
                  <span className={styles.codeChip} key={s}>
                    {s}
                  </span>
                ))}
              </div>
            </>
          )}
        </div>
      </Section>
    </>
  )
}

export default RecoveryTab
