-- Eidolon — Local Code Intelligence for Neovim
-- Connects to the Eidolon Go backend via HTTP

local M = {}

local config = {
  api_url = "http://localhost:8080",
  debounce_ms = 300,
  max_tokens = 64,
  temperature = 0.2,
  enabled = true,
}

local debounce_timer = nil
local current_suggestion = nil
local ns_id = vim.api.nvim_create_namespace("eidolon")

-- Setup function
function M.setup(opts)
  config = vim.tbl_deep_extend("force", config, opts or {})

  -- health check on startup
  M.check_health()

  -- set up autocommands
  local group = vim.api.nvim_create_augroup("Eidolon", { clear = true })

  vim.api.nvim_create_autocmd("TextChangedI", {
    group = group,
    callback = function()
      if config.enabled then
        M.debounced_complete()
      end
    end,
  })

  vim.api.nvim_create_autocmd("InsertLeave", {
    group = group,
    callback = function()
      M.clear_suggestion()
  end,
  })

  -- keymaps
  vim.keymap.set("i", "<Tab>", function()
    if current_suggestion then
      return M.accept_suggestion()
    end
    return "<Tab>"
  end, { expr = true, desc = "Accept Eidolon suggestion" })

  vim.keymap.set("i", "<Esc>", function()
    if current_suggestion then
      M.clear_suggestion()
      return ""
    end
    return "<Esc>"
  end, { expr = true, desc = "Dismiss Eidolon suggestion" })

  vim.keymap.set("n", "<leader>et", M.toggle, { desc = "Toggle Eidolon" })
  vim.keymap.set("n", "<leader>ed", M.open_dashboard, { desc = "Open Eidolon Dashboard" })

  vim.notify("Eidolon loaded", vim.log.levels.INFO)
end

-- Debounced completion trigger
function M.debounced_complete()
  if debounce_timer then
    debounce_timer:stop()
    debounce_timer:close()
    debounce_timer = nil
  end

  debounce_timer = vim.loop.new_timer()
  debounce_timer:start(config.debounce_ms, 0, vim.schedule_wrap(function()
    M.request_completion()
  end))
end

-- Request completion from Eidolon backend
function M.request_completion()
  local buf = vim.api.nvim_get_current_buf()
  local cursor = vim.api.nvim_win_get_cursor(0)
  local row = cursor[1] - 1
  local col = cursor[2]

  -- get all lines and compute offset
  local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  local content = table.concat(lines, "\n")

  -- compute cursor offset
  local offset = 0
  for i = 1, row do
    offset = offset + #lines[i] + 1
  end
  offset = offset + col

  if #content < 10 then return end

  local request_id = "nvim-" .. tostring(vim.loop.now())
  local file_path = vim.api.nvim_buf_get_name(buf)
  local lang = vim.bo[buf].filetype

  local body = vim.json.encode({
    request_id = request_id,
    file_path = file_path,
    language_id = lang,
    content = content,
    cursor_offset = offset,
    timestamp = vim.loop.now(),
  })

  -- send keystroke to backend
  vim.system({
    "curl", "-s", "-X", "POST",
    config.api_url .. "/api/keystroke",
    "-H", "Content-Type: application/json",
    "-d", body,
  }, { text = true }, function(result)
    if result.code ~= 0 then return end

    -- poll for completion
    vim.schedule(function()
      M.poll_completion(request_id, row, col)
    end)
  end)
end

-- Poll SSE stream for completion
function M.poll_completion(request_id, row, col)
  local full_text = ""

  vim.system({
    "curl", "-s", "--max-time", "60",
    config.api_url .. "/api/completions/stream",
  }, { text = true }, function(result)
    if result.code ~= 0 or not result.stdout then return end

    -- parse NDJSON response
    for line in result.stdout:gmatch("[^\n]+") do
      if line:match("^data: ") then
        local json_str = line:sub(7)
        local ok, data = pcall(vim.json.decode, json_str)
        if ok and data and data.request_id == request_id then
          full_text = full_text .. (data.token or "")
          if data.done then break end
        end
      end
    end

    if full_text ~= "" then
      vim.schedule(function()
        M.show_suggestion(full_text, row, col, request_id)
      end)
    end
  end)
end

-- Show ghost text suggestion
function M.show_suggestion(text, row, col, request_id)
  M.clear_suggestion()

  -- only show first line as virtual text
  local first_line = text:match("([^\n]*)")
  if not first_line or first_line == "" then return end

  vim.api.nvim_buf_set_extmark(0, ns_id, row, col, {
    virt_text = {{ first_line, "Comment" }},
    virt_text_pos = "inline",
    hl_mode = "combine",
  })

  current_suggestion = {
    text = text,
    row = row,
    col = col,
    request_id = request_id,
  }
end

-- Accept the current suggestion
function M.accept_suggestion()
  if not current_suggestion then return "<Tab>" end

  local text = current_suggestion.text
  local request_id = current_suggestion.request_id

  M.clear_suggestion()

  -- insert the completion text
  local lines = vim.split(text, "\n", { plain = true })
  vim.api.nvim_put(lines, "c", true, true)

  -- send accept feedback
  vim.system({
    "curl", "-s", "-X", "POST",
    config.api_url .. "/api/feedback",
    "-H", "Content-Type: application/json",
    "-d", vim.json.encode({ request_id = request_id, action = "accept" }),
  }, { text = true }, function() end)

  return ""
end

-- Clear the ghost text
function M.clear_suggestion()
  vim.api.nvim_buf_clear_namespace(0, ns_id, 0, -1)
  if current_suggestion then
    -- send reject feedback
    local request_id = current_suggestion.request_id
    vim.system({
      "curl", "-s", "-X", "POST",
      config.api_url .. "/api/feedback",
      "-H", "Content-Type: application/json",
      "-d", vim.json.encode({ request_id = request_id, action = "reject" }),
    }, { text = true }, function() end)
  end
  current_suggestion = nil
end

-- Toggle Eidolon on/off
function M.toggle()
  config.enabled = not config.enabled
  local status = config.enabled and "enabled" or "disabled"
  vim.notify("Eidolon " .. status, vim.log.levels.INFO)
end

-- Open dashboard in browser
function M.open_dashboard()
  vim.system({ "open", "http://localhost:3000" }, {}, function() end)
end

-- Health check
function M.check_health()
  vim.system({
    "curl", "-s", "--max-time", "2",
    config.api_url .. "/api/health",
  }, { text = true }, function(result)
    if result.code ~= 0 then
      vim.schedule(function()
        vim.notify("Eidolon: server not running at " .. config.api_url, vim.log.levels.WARN)
      end)
    end
  end)
end

return M
