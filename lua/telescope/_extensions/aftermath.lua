local am = require("aftermath.api")
local utils = require("aftermath.utils")

local has_telescope, telescope = pcall(require, "telescope")

if not has_telescope then
	utils.error("This extension requires Telescope.nvim (https://github.com/nvim-telescope/telescope.nvim)")
end

local pickers = require("telescope.pickers")
local finders = require("telescope.finders")

local function index(opts)
	opts = opts or {}

	pickers
		.new(opts, {
			prompt_title = "All",
			finder = finders.new_table({
				results = am.getall(),
				entry_maker = function(entry)
					local name = utils.path2zettel(entry)
					return {
						value = entry,
						display = name,
						ordinal = name,
					}
				end,
			}),
			sorter = require("telescope.config").values.generic_sorter(opts),
		})
		:find()
end

return telescope.register_extension({
	exports = {
		index = index,
	},
})
