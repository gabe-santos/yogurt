# Three panes, one reading surface

The reader is a Collection List, an Entry List, and a Reading Pane. The Reading Pane replaced a right-side Sheet drawer, and before there is room for three columns it is the same component rendered as an overlay over the Entry List rather than a Sheet — deliberately unlike the sidebar's mobile behaviour in web/src/lib/components/ui/sidebar/sidebar.svelte, which does use a Sheet. A Sheet for narrow and a pane for wide would mean every future reading feature is built and tested twice; one component with two container styles keeps the reading surface single.
