local M = {}

function M.check()
  vim.health.start("Eidolon")

  -- check neovim version
  if vim.fn.has("nvim-0.10") == 1 then
    vim.health.ok("Neovim >= 0.10")
  else
    vim.health.warn("Neovim >= 0.10 recommended")
  end

  -- check curl
  if vim.fn.executable("curl") == 1 then
    vim.health.ok("curl found")
  else
    vim.health.error("curl not found — required for API calls")
  end

  -- check eidolon server
  local result = vim.system(
    { "curl", "-s", "--max-time", "2", "http://localhost:8080/api/health" },
    { text = true }
  ):wait()

  if result.code == 0 and result.stdout:match("ok") then
    vim.health.ok("Eidolon server running at localhost:8080")
  else
    vim.health.warn("Eidolon server not running — start with: go run cmd/server/main.go")
  end

  -- check inference server
  local sock = vim.loop.fs_stat("/tmp/eidolon.sock")
  if sock then
    vim.health.ok("Infer server socket found at /tmp/eidolon.sock")
  else
    vim.health.warn("Inference server not running — start with: python3 ml/inference_server.py")
  end
end

return M
