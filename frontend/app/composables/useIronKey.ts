// IronKey / USB file helpers for Emergency Recovery Key shares.
// Uses the File System Access API when available (Chrome/Edge 86+);
// falls back to <a download> / <input type=file> for Firefox / Safari.

export function useIronKey() {
  const hasFsApi = typeof window !== 'undefined' && 'showSaveFilePicker' in window

  async function exportShare(shareString: string, index: number, total: number): Promise<boolean> {
    const filename = `scutum-erk-share-${index}-of-${total}.erk`

    if (hasFsApi) {
      try {
        const handle = await (window as any).showSaveFilePicker({
          suggestedName: filename,
          types: [{ description: 'Scutum ERK Share', accept: { 'text/plain': ['.erk'] } }],
        })
        const writable = await handle.createWritable()
        await writable.write(shareString)
        await writable.close()
        return true
      } catch (e: any) {
        if (e.name === 'AbortError') return false
        throw e
      }
    }

    // Fallback: trigger browser download
    const a = document.createElement('a')
    a.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(shareString)
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    return true
  }

  async function importShare(): Promise<string | null> {
    if (hasFsApi) {
      try {
        const [handle] = await (window as any).showOpenFilePicker({
          types: [{ description: 'Scutum ERK Share', accept: { 'text/plain': ['.erk', '.txt'] } }],
          multiple: false,
        })
        const file = await handle.getFile()
        return (await file.text()).trim()
      } catch (e: any) {
        if (e.name === 'AbortError') return null
        throw e
      }
    }

    return new Promise((resolve) => {
      const input = document.createElement('input')
      input.type = 'file'
      input.accept = '.erk,.txt'
      input.onchange = async () => {
        const file = input.files?.[0]
        if (!file) { resolve(null); return }
        resolve((await file.text()).trim())
      }
      input.click()
    })
  }

  return { hasFsApi, exportShare, importShare }
}
