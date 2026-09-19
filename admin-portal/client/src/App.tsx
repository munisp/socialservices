import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import NotFound from "@/pages/NotFound";
import { Route, Switch } from "wouter";
import ErrorBoundary from "./components/ErrorBoundary";
import { ThemeProvider } from "./contexts/ThemeContext";
import Home from "./pages/Home";
import ProgramsPage from "./pages/ProgramsPage";
import ProgramDetailPage from "./pages/ProgramDetailPage";
import FeatureFlagsPage from "./pages/FeatureFlagsPage";
import DisbursementsPage from "./pages/DisbursementsPage";
import UsersPage from "./pages/UsersPage";
import MCCImportPage from "./pages/MCCImportPage";
import MCCManagementPage from "@/pages/MCCManagementPage";
import MCCAnalyticsPage from "@/pages/MCCAnalyticsPage";
import ReportsPage from "@/pages/ReportsPage";
import TemplatesPage from "@/pages/TemplatesPage";
import ApprovalsPage from "@/pages/ApprovalsPage";
import SettingsPage from "@/pages/SettingsPage";
import Beneficiaries from "./pages/Beneficiaries";
import Transactions from "./pages/Transactions";
import DataExport from "./pages/DataExport";
import MiddlewareDashboard from "./pages/MiddlewareDashboard";
import WorkflowMonitoring from "./pages/WorkflowMonitoring";
import {
  OfflineSyncPage,
  NationalIDPage,
  PMTPage,
  InteropPage,
  LoadTestPage,
} from "./pages/world-class";

function Router() {
  return (
    <Switch>
      <Route path={"/"} component={Home} />
      <Route path={"/programs"} component={ProgramsPage} />
      <Route path={"/programs/:id"} component={ProgramDetailPage} />
      <Route path={"/feature-flags"} component={FeatureFlagsPage} />
      <Route path={"/disbursements"} component={DisbursementsPage} />
      <Route path={"/users"} component={UsersPage} />
      <Route path={"/mcc-import"} component={MCCImportPage} />
        <Route path="/mcc-management" component={MCCManagementPage} />
        <Route path="/mcc-analytics" component={MCCAnalyticsPage} />
      <Route path="/reports" component={ReportsPage} />
      <Route path="/templates" component={TemplatesPage} />
      <Route path="/approvals" component={ApprovalsPage} />
      <Route path="/settings" component={SettingsPage} />
      <Route path="/beneficiaries" component={Beneficiaries} />
      <Route path="/transactions" component={Transactions} />
      <Route path="/data-export" component={DataExport} />
      <Route path="/middleware" component={MiddlewareDashboard} />
      <Route path="/workflow-monitoring" component={WorkflowMonitoring} />
      <Route path="/offline-sync" component={OfflineSyncPage} />
      <Route path="/national-id" component={NationalIDPage} />
      <Route path="/pmt" component={PMTPage} />
      <Route path="/interop" component={InteropPage} />
      <Route path="/load-test" component={LoadTestPage} />
      <Route path="/404" component={NotFound} />
      <Route component={NotFound} />
    </Switch>
  );
}

function App() {
  return (
    <ErrorBoundary>
      <ThemeProvider defaultTheme="light">
        <TooltipProvider>
          <Toaster />
          <Router />
        </TooltipProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}

export default App;
