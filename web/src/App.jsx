import { useState, useEffect, useRef } from "react"
import { motion, AnimatePresence } from "framer-motion"
import { Activity, Zap, Settings, Terminal, Radio, CheckCircle, Clock, TrendingUp } from "lucide-react"
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, AreaChart, Area } from "recharts"

export default function App() {
  const [tab, setTab] = useState("feed")
  const [status, setStatus] = useState(null)
  const [completions, setCompletions] = useState([])
  const [metrics, setMetrics] = useState([])
  const [testInput, setTestInput] = useState("")
  const [testResult, setTestResult] = useState("")
  const [streamingTokens, setStreamingTokens] = useState({})
  const feedRef = useRef(null)

  useEffect(() => {
    const fetchStatus = () =>
      fetch("/api/status").then(r => r.json()).then(setStatus).catch(() => {})
    fetchStatus()
    const t = setInterval(fetchStatus, 5000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    const es = new EventSource("/api/completions/stream")
    let current = null

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)

        if (!current || current.request_id !== data.request_id) {
          current = {
            request_id: data.request_id,
            file: data.file_path || "unknown",
            text: "",
            ts: Date.now(),
            done: false,
            tokenCount: 0,
            firstTokenMs: null,
            startTs: Date.now(),
          }
        }

        if (current.firstTokenMs === null && data.token) {
          current.firstTokenMs = Date.now() - current.startTs
        }

        current.text += data.token
        current.tokenCount += 1

        // update streaming display
        setStreamingTokens(prev => ({ ...prev, [current.request_id]: current.text }))

        if (data.done) {
          current.done = true
          const c = { ...current }
          setCompletions(prev => [c, ...prev].slice(0, 50))
          setStreamingTokens(prev => {
            const next = { ...prev }
            delete next[c.request_id]
            return next
          })
          setMetrics(prev => [...prev, {
            t: prev.length,
            tokens: c.tokenCount,
            latency: c.firstTokenMs || 0,
          }].slice(-30))
          current = null
        }
      } catch {}
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
    setTestResult("queued — watch Live Feed")
    setTab("feed")
  }

  const acceptRate = completions.length > 0
    ? Math.round((completions.filter(c => c.done).length / completions.length) * 100)
    : 0

  const avgLatency = metrics.length > 0
    ? Math.round(metrics.reduce((a, b) => a + b.latency, 0) / metrics.length)
    : 0

  const tabs = [
    { id: "feed",    label: "Live Feed", icon: Activity },
    { id: "test",    label: "Test",      icon: Terminal },
    { id: "metrics", label: "Metrics",   icon: Zap      },
    { id: "config",  label: "Config",    icon: Settings },
  ]

  return (
    <div className="min-h-screen bg-[#0A0A0F] text-gray-100 font-mono">

      {/* Header */}
      <div className="border-b border-white/5 px-6 py-4 flex items-center justify-between backdrop-blur-sm sticky top-0 z-10 bg-[#0A0A0F]/90">
        <div className="flex items-center gap-3">
          <div className="relative">
            <div className="w-2 h-2 rounded-full bg-indigo-500" />
            <div className="w-2 h-2 rounded-full bg-indigo-500 absolute inset-0 animate-ping opacity-40" />
          </div>
          <span className="text-indigo-400 font-bold tracking-[0.2em] text-sm">EIDOLON</span>
          <span className="text-white/20 text-xs">local code intelligence</span>
        </div>
        <div className="flex items-center gap-6 text-xs">
          {status && (
            <>
              <span className="flex items-center gap-1.5 text-white/40">
                <Radio size={9} className="text-green-400" />
                {status.model}
              </span>
              <div className="flex items-center gap-1.5 text-white/40">
                <div className="w-1.5 h-1.5 rounded-full bg-green-400" />
                kafka connected
              </div>
            </>
          )}
        </div>
      </div>

      {/* Stats bar */}
      <div className="border-b border-white/5 px-6 py-2 flex items-center gap-8">
        {[
          { label: "completions", value: completions.length, icon: Activity },
          { label: "avg latency", value: avgLatency ? avgLatency + "ms" : "--", icon: Clock },
          { label: "accept rate", value: acceptRate + "%", icon: TrendingUp },
          { label: "streaming", value: Object.keys(streamingTokens).length, icon: Zap },
        ].map(({ label, value, icon: Icon }) => (
          <div key={label} className="flex items-center gap-2">
            <Icon size={11} className="text-indigo-400/60" />
            <span className="text-white/30 text-xs">{label}</span>
            <span className="text-indigo-300 text-xs font-bold">{value}</span>
          </div>
        ))}
      </div>

      {/* Tabs */}
      <div className="border-b border-white/5 px-6 flex gap-1">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={"flex items-center gap-2 py-3 px-3 text-xs border-b-2 transition-all " + (
              tab === id
                ? "border-indigo-500 text-indigo-400"
                : "border-transparent text-white/30 hover:text-white/60"
            )}
          >
            <Icon size={12} />
            {label}
          </button>
        ))}
      </div>

      <div className="p-6">

        {/* Live Feed */}
        {tab === "feed" && (
          <div className="space-y-3 max-w-4xl" ref={feedRef}>

            {/* Streaming cards */}
            <AnimatePresence>
              {Object.entries(streamingTokens).map(([reqId, text]) => (
                <motion.div
                  key={reqId}
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  className="bg-indigo-950/30 border border-indigo-500/20 rounded-xl p-4"
                >
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-xs text-indigo-400/60">generating</span>
                    <div className="flex items-center gap-1.5">
                      <div className="w-1.5 h-1.5 rounded-full bg-indigo-400 animate-pulse" />
                      <span className="text-xs text-indigo-400/60">streaming</span>
                    </div>
                  </div>
                  <pre className="text-sm text-indigo-200 whitespace-pre-wrap break-all leading-relaxed">
                    {text}<span className="animate-pulse text-indigo-400">▋</span>
                  </pre>
                </motion.div>
              ))}
            </AnimatePresence>

            {/* Completed cards */}
            {completions.length === 0 && Object.keys(streamingTokens).length === 0 && (
              <div className="text-center py-24">
                <div className="text-white/10 text-6xl mb-4">⟁</div>
                <p className="text-white/20 text-sm">waiting for completions</p>
                <p className="text-white/10 text-xs mt-1">go to Test tab and send some code</p>
              </div>
            )}

            <AnimatePresence>
              {completions.map((c, i) => (
                <motion.div
                  key={c.request_id + i}
                  initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: i === 0 ? 0 : 0 }}
                  className="bg-white/[0.02] border border-white/5 rounded-xl p-4 hover:border-white/10 transition-colors"
                >
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-white/20 font-mono">{c.file}</span>
                      {c.firstTokenMs && (
                        <span className="text-xs text-white/10">· {c.firstTokenMs}ms first token</span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle size={11} className="text-green-400/60" />
                      <span className="text-xs text-white/20">{new Date(c.ts).toLocaleTimeString()}</span>
                    </div>
                  </div>
                  <pre className="text-sm text-indigo-20/80 whitespace-pre-wrap break-all leading-relaxed border-l-2 border-indigo-500/20 pl-3">
                    {c.text}
                  </pre>
                  <div className="mt-3 flex items-center gap-4">
                    <span className="text-xs text-white/10">{c.tokenCount} tokens</span>
                  </div>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        )}

        {/* Test */}
        {tab === "test" && (
          <div className="max-w-2xl space-y-4">
            <div className="bg-white/[0.02] border border-white/5 rounded-xl p-4">
              <p className="text-xs text-white/30 mb-4">
                Type code below — cursor position is at the end. Eidolon will complete it.
              </p>
              <textarea
                className="w-full bg-transparent border border-white/10 rounded-lg p-4 text-sm text-indigo-200 font-mono focus:outline-none focus:border-indigo-500/50 resize-none placeholder-white/10"
                ws={10}
                value={testInput}
                onChange={e => setTestInput(e.target.value)}
                placeholder="func add(a int, b int) int { return"
              />
              <div className="flex items-center justify-between mt-3">
                <button
                  onClick={sendKeystroke}
                  className="bg-indigo-600/80 hover:bg-indigo-500 text-white text-xs px-5 py-2 rounded-lg transition-colors"
                >
                  Send to Eidolon →
                </button>
                {testResult && (
                  <motion.p
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="text-xs text-green-400/70"
                  >
                    {testResult}
                  </motion.p>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Metrics */}
        {tab === "metrics" && (
          <div className="space-y-4 max-w-4xl">
          <div className="grid grid-cols-4 gap-3">
              {[
                { label: "Total completions", value: completions.length },
                { label: "Avg first token", value: avgLatency ? avgLatency + "ms" : "--" },
                { label: "Accept rate", value: acceptRate + "%" },
                { label: "Model", value: status?.model ?? "--" },
              ].map(({ label, value }) => (
                <div key={label} className="bg-white/[0.02] border border-white/5 rounded-xl p-4">
                  <p className="text-xs text-white/20 mb-2">{label}</p>
                  <p className="text-xl text-indigo-400 font-bold">{String(value)}</p>
                </div>
              ))}
            </div>

            <div className="bg-white/[0.02] border border-white/5 rounded-xl p-4">
              <p className="text-xs text-white/20 mb-4">Tokens per completion</p>
              {metrics.length > 1 ? (
                <ResponsiveContainer width="100%" height={180}>
                  <AreaChart data={metrics}>
                    <defs>
                      <linearGradient id="tokenGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <XAxis dataKey="t" hide />
                    <YAxis tick={{ fontSize: 10, fill: "#ffffff20" }} axisLine={false} tickLine={false} />
                    <Tooltip
                      contentStyle={{ background: "#0d0d1a", border: "1px solid #ffffff10", fontSize: 11, borderRadius: 8 }}
                      labelFormatter={() => ""}
                    />
                    <Area type="monotone" dataKey="tokens" stroke="#6366f1" strokeWidth={2} fill="url(#tokenGrad)" dot={false} />
                  </AreaChart>
                </ResponsiveContainer>
              ) : (
                <div className="text-center py-12 text-white/10 text-sm">send some completions first</div>
              )}
            </div>

            <div className="bg-white/[0.02] border border-white/5 rounded-xl p-4">
              <p className="text-xs text-white/20 mb-4">First token latency (ms)</p>
              {metrics.length > 1 ? (
                <ResponsiveContainer width="100%" height={180}>
                  <LineChart data={metrics}>
                    <XAxis dataKey="t" hide />
                    <YAxis tick={{ fontSize: 10, fill: "#ffffff20" }} axisLine={false} tickLine={false} />
                    <Tooltip
                      contentStyle={{ background: "#0d0d1a", border: "1px solid #ffffff10", fontSize: 11, borderRadius: 8 }}
                      labelFormatter={() => ""}
                    />
                    <Line type="monotone" dataKey="latency" stroke="#14b8a6" strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <div className="text-center py-12 text-white/10 text-sm">no latency data yet</div>
              )}
            </div>
          </div>
        )}

        {/* Config */}
        {tab === "config" && (
          <div className="max-w-md space-y-3">
            {[
              { title: "MODEL", rows: [
                { label: "Active model", value: status?.model ?? "loading..." },
                { label: "Adapter", value: status?.adapter ?? "base" },
              ]},
              { title: "KAFKA", rows: [
                { label: "Bootstrap servers", value: "localhost:9092", green: true },
                { label: "Status", value: "connected", green: true },
              ]},
              { title: "INFERENCE", rows: [
                { label: "Socket", value: "/tmp/eidolon.sock" },
                { label: "Temperature", value: "0.2" },
                { label: "Max tokens", value: "256" },
              ]},
            ].map(({ title, rows }) => (
              <div key={title} className="bg-white/[0.02] border border-white/5 rounded-xl p-4">
                <p className="text-xs text-white/20 font-bold mb-3 tracking-widest">{title}</p>
                <div className="space-y-2">
                  {rows.map(({ label, value, green }) => (
                    <div key={label} className="flex items-center justify-between">
                      <span className="text-xs text-white/30">{label}</span>
                      <span className={"text-xs font-mono " + (green ? "text-green-400/70" : "text-indigo-300/70")}>
                        {value}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

      </div>
    </div>
  )
}
