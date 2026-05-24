-- Eidolon plugin entry point
if vim.g.eidolon_loaded then return end
vim.g.eidolon_loaded = true

-- Auto-setup with defaults if not configured manually
vim.api.nvim_create_autocmd("VimEnter", {
  once = true,
  callback = function()
    if not vim.g.eidolon_setup_called then
      require("eidolon").setup()
    end
  end,
})
