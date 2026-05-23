import { useState, useEffect } from "react"
import { Activity, Zap, Settings, Terminal, Radio, CheckCircle } from "lucide-react"
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts"

export default function App() {
  const [tab, setTab] = useState("feed")
  const [status, setStatus] = useState(null)
  const [completions, setCompletions] = useState([])
  const [metrics, setMetrics] = useState([])
  const [testInput, setTestInput] = useState("")
  const [testResult, setTestResult] = useState("")

  useEffect(() => {
    const fetch_status = () =>
      fetch("/api/status").then(r => r.json()).then(setStatus).catch(() => {})
    fetch_status()
    const t = setInterval(fetch_status, 5000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    const es = new EventSource("/api/completions/stream")
    let current = null
    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (!current || current.request_id !== data.request_id) {
          current = { request_id: data.request_id, file: data.file_path, text: "", ts: Date.now(), done: false }
        }
        current.text += data.token
        if (data.done) {
          current.done = true
          const c = { ...current }
          setCompletions(prev => [c, ...prev].slice(0, 50))
          setMetrics(prev => [...prev, { t: prev.length, tokens: c.text.split(" ").length }].slice(-30))
          current = null
        }
      } catch(err) {}
    }
    return () => es.close()
  }, [])

  const sendKeystroke = async () => {
    if (!testInput.trim()) return
    setTestResult("sending...")
    await fetch("/api/keystroke", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        request_id: "ui-" + Date.now(),
        file_path: "test.go",
        language_id: "go",
        content: testInput,
        cursor_offset: testInput.length,
        timestamp: Date.now(),
      })
    })
    setTestResult("sent — watch the Live Feed tab")
  }

  const tabs = [
    { id: "feed",    label: "Live Feed", icon: Activity },
    { id: "test",    label: "Test",      icon: Terminal },
    { id: "metrics", label: "Metrics",   icon: Zap      },
    { id: "config",  label: "Config",    icon: Settings },
  ]

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 font-mono">
      <div className="border-b border-gray-800 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-2 h-2 rounded-full bg-indigo-500 animate-pulse" />
          <span className="text-indigo-400 font-bold tracking-widest text-sm">EIDOLON</span>
          <span className="text-gray-600 text-xs">local code intelligence</span>
        </div>
        <div className="flex items-center gap-4 text-xs text-gray-500">
          {status && (
            <span className="flex items-center gap-1">
            <Radio size={10} className="text-green-400" />
              <span>{status.model}</span>
            </span>
          )}
        </div>
      </div>

      <div className="border-b border-gray-800 px-6 flex gap-6">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={"flex items-center gap-2 py-3 text-xs border-b-2 transition-colors " + (
              tab === id ? "border-indigo-500 text-indigo-400" : "border-transparent text-gray-500 hover:text-gray-300"
            )}
          >
            <Icon size={13} />
            {label}
          </button>
        ))}
      </div>

      <div className="p-6">
        {tab === "feed" && (
          <div className="space-y-3">
            <div className="flex items-center justify-between mb-4">
              <span className="text-xs text-gray-500">{completions.length} completions received</span>
              <span className="text-xs text-indigo-400 animate-pulse">live</span>
            </div>
            {completions.length === 0 && (
              <div className="text-center py-20 text-gray-700 text-sm">
                Waiting for completions — send a keystroke from the Test tab
              </div>
            )}
            {completions.map((c, i) => (
              <div key={i} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs text-gray-500">{c.file}</span>
                  <div className="flex items-center gap-2">
                    {c.done && <CheckCircle size={12} className="text-green-400" />}
                    <span className="text-xs text-gray-600">{new Date(c.ts).toLocaleTimeString()}</span>
                  </div>
                </div>
                <pre className="text-sm text-indigo-300 whitespace-pre-wrap break-all leading-relaxed">
                  {c.text || "generating..."}
                </pre>
            </div>
            ))}
          </div>
        )}

        {tab === "test" && (
          <div className="max-w-2xl space-y-4">
            <p className="text-xs text-gray-500 mb-4">Type code and send it to Eidolon. Watch completions appear in the Live Feed tab.</p>
            <textarea
              className="w-full bg-gray-900 border border-gray-700 rounded-lg p-4 text-sm text-gray-100 font-mono focus:outline-none focus:border-indigo-500 resize-none"
              rows={8}
              value={testInput}
              onChange={e => setTestInput(e.target.value)}
              placeholder="func add(a int, b int) int {
	return"
            />
            <button
              onClick={sendKeystroke}
              className="bg-indigo-600 hover:bg-indigo-500 text-white text-xs px-6 py-2.5 rounded-lg transition-colors"
            >
              Send to Eidolon
            </button>
            {testResult && <p className="text-xs text-green-400 mt-2">{testResult}</p>}
          </div>
        )}

        {tab === "metrics" && (
          <div className="space-y-6">
            <div className="grid grid-cols-3 gap-4">
              {[
                { label: "Completions", value: String(completions.length) },
                { label: "Model",       value: status ? status.model   : "-" },
                { label: "Adapter",     value: status ? status.adapter : "-" },
              ].map(({ label, value }) => (
                <div key={label} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
                  <p className="text-xs text-gray-500 mb-1">{label}</p>
                  <p className="text-lg text-indigo-400 font-bold">{value}</p>
                </div>
              ))}
            </div>
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
              <p className="text-xs text-gray-500 mb-4">Tokens per completion</p>
              {metrics.length > 1 ? (
                <ResponsiveContainer width="100%" height={200}>
                  <LineChart data={metrics}>
                    <XAxis dataKey="t" hide />
                    <YAxis tick={{ fontSize: 10, fill: "#6b7280" }} />
                    <Tooltip contentStyle={{ background: "#111827", border: "1px solid #374151", fontSize: 11 }} />
                    <Line type="monotone" dataKey="tokens" stroke="#6366f1" strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <p className="text-center text-gray-700 text-sm py-10">No data yet</p>
              )}
            </div>
          </div>
        )}

        {tab === "config" && (
          <div className="max-w-md space-y-6">
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-4">
              <p className="text-xs text-gray-400 font-bold">MODEL</p>
              <div>
                <p className="text-xs text-gray-500 mb-1">Active model</p>
                <div className="bg-gray-800 rounded px-3 py-2 text-sm text-indigo-300">{status ? status.model : "loading..."}</div>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Adapter</p>
                <div className="bg-gray-800 rounded px-3 py-2 text-sm text-indigo-300">{status ? status.adapter : "base"}</div>
              </div>
            </div>
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-4">
              <p className="text-xs text-gray-400 font-bold">KAFKA</p>
              <div>
                <p className="text-xs text-gray-500 mb-1">Bootstrap servers</p>
                <div className="bg-gray-800 rounded px-3 py-2 text-sm text-green-400">localhost:9092</div>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full bg-green-400" />
                <span className="text-sm text-green-400">connected</span>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
