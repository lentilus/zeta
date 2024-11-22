local api = require("aftermath.api")
local utils = require("aftermath.utils")

local has_telescope, telescope = pcall(require, "telescope")

if not has_telescope then
	utils.error("This extension requires Telescope.nvim (https://github.com/nvim-telescope/telescope.nvim)")
end

local pickers = require("telescope.pickers")
local finders = require("telescope.finders")

local function entries(entry)
	local name = utils.path2zettel(entry)
	return {
		value = entry,
		display = name,
		ordinal = name,
	}
end

local function index(opts)
	opts = opts or {}
	pickers
		.new(opts, {
			prompt_title = "All",
			finder = finders.new_table({
				results = api.getall(),
				entry_maker = entries,
			}),
			sorter = require("telescope.config").values.generic_sorter(opts),
		})
		:find()
end

local function children(opts)
	opts = opts or {}
	local file = utils.current_file()
	pickers
		.new(opts, {
			prompt_title = "Children",
			finder = finders.new_table({
				results = api.get_children(file),
				entry_maker = entries,
			}),
			sorter = require("telescope.config").values.generic_sorter(opts),
		})
		:find()
end

local function parents(opts)
	opts = opts or {}
	local file = utils.current_file()
	pickers
		.new(opts, {
			prompt_title = "Parents",
			finder = finders.new_table({
				results = api.get_parents(file),
				entry_maker = entries,
			}),
			sorter = require("telescope.config").values.generic_sorter(opts),
		})
		:find()
end

return telescope.register_extension({
	exports = {
		index = index,
		children = children,
		parents = parents,
	},
})
