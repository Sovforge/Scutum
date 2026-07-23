import { Route, Routes } from 'react-router-dom'
import Landing from './pages/Landing/Landing'
import Setup from './pages/Setup/Setup'
import Login from './pages/Login/Login'
import About from './pages/About/About'
import WhatsNew from './pages/WhatsNew/WhatsNew'
import FAQ from './pages/FAQ/FAQ'
import ForgotPassword from './pages/ForgotPassword/ForgotPassword'
import Dashboard from './pages/Dashboard/Dashboard'
import TV from './pages/TV/TV'
import Nodes from './pages/Nodes/Nodes'
import Containers from './pages/Containers/Containers'
import ContainerDetail from './pages/Containers/ContainerDetail'
import Kubernetes from './pages/Kubernetes/Kubernetes'
import PodDetail from './pages/Kubernetes/PodDetail'
import Storage from './pages/Storage/Storage'
import Network from './pages/Network/Network'
import Observability from './pages/Observability/Observability'
import Terminal from './pages/Terminal/Terminal'
import GitOps from './pages/GitOps/GitOps'
import Plugins from './pages/Plugins/Plugins'
import AuditLog from './pages/AuditLog/AuditLog'
import Account from './pages/Account/Account'
import Settings from './pages/Settings/Settings'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/setup" element={<Setup />} />
      <Route path="/login" element={<Login />} />
      <Route path="/about" element={<About />} />
      <Route path="/whats-new" element={<WhatsNew />} />
      <Route path="/faq" element={<FAQ />} />
      <Route path="/forgot" element={<ForgotPassword />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/tv" element={<TV />} />
      <Route path="/nodes" element={<Nodes />} />
      <Route path="/containers" element={<Containers />} />
      <Route path="/containers/:id" element={<ContainerDetail />} />
      <Route path="/kubernetes" element={<Kubernetes />} />
      <Route path="/kubernetes/pods/:namespace/:name" element={<PodDetail />} />
      <Route path="/storage" element={<Storage />} />
      <Route path="/network" element={<Network />} />
      <Route path="/observability" element={<Observability />} />
      <Route path="/terminal" element={<Terminal />} />
      <Route path="/gitops" element={<GitOps />} />
      <Route path="/plugins" element={<Plugins />} />
      <Route path="/audit" element={<AuditLog />} />
      <Route path="/account" element={<Account />} />
      <Route path="/settings" element={<Settings />} />
    </Routes>
  )
}

export default App
