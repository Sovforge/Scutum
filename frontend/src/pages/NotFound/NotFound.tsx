import { Link } from 'react-router-dom'
import { Compass } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import Button from '../../components/ui/Button/Button'
import styles from './NotFound.module.css'

// Catch-all for any unmatched path — including bookmarks to routes the old
// Nuxt frontend had that this rewrite restructured (most Settings sub-pages
// collapsed into tabs under one /settings route; /auth/login and
// /auth/forgot moved to /login and /forgot). Without this route, an
// unmatched path previously rendered nothing at all — a blank white page.
function NotFound() {
  return (
    <AppShell title="Not Found">
      <div className={styles.page}>
        <EmptyState
          icon={Compass}
          title="Page not found"
          description="This URL doesn't match anything — it may have moved. Settings pages, for instance, are tabs under Settings now instead of separate URLs."
          action={
            <Button as={Link} to="/dashboard">
              Back to Dashboard
            </Button>
          }
        />
      </div>
    </AppShell>
  )
}

export default NotFound
