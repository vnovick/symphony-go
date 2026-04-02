import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Per-page error boundary that catches rendering errors within a single route
 * and displays a recovery UI with a link back to the Dashboard.
 * Unlike the root ErrorBoundary, this keeps the app shell (sidebar, header)
 * intact so the user can navigate away without a full page reload.
 */
export class PageErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[Symphony] Page render error', error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="flex flex-col items-center justify-center gap-4 py-20 text-center">
        <h2 className="text-xl font-semibold text-theme-danger">
          This page crashed
        </h2>
        <p className="max-w-md text-sm text-theme-muted">
          An unexpected error occurred while rendering this page.
        </p>
        <pre className="max-w-lg overflow-auto rounded-lg p-4 text-left text-xs border border-theme-line bg-theme-bg-soft text-theme-text-secondary">
          {error.message}
        </pre>
        <a
          href="/"
          className="rounded-lg px-4 py-2 text-sm font-medium text-white bg-theme-accent hover:opacity-90 transition-opacity"
        >
          Go to Dashboard
        </a>
      </div>
    );
  }
}
