package com.zupu.app

import android.content.Context
import android.graphics.Color
import android.view.View
import android.view.ViewGroup
import android.widget.BaseExpandableListAdapter
import android.widget.TextView

/** 可折叠的抽屉菜单：分组默认收起，点分组标题展开/收起 */
class DrawerAdapter(
    private val ctx: Context,
    private val groups: List<Group>
) : BaseExpandableListAdapter() {

    data class Item(val title: String, val route: String?, val tag: String? = null)
    data class Group(val title: String, val items: List<Item>)

    override fun getGroupCount() = groups.size
    override fun getChildrenCount(groupPosition: Int) = groups[groupPosition].items.size
    override fun getGroup(groupPosition: Int) = groups[groupPosition]
    override fun getChild(groupPosition: Int, childPosition: Int) = groups[groupPosition].items[childPosition]
    override fun getGroupId(groupPosition: Int) = groupPosition.toLong()
    override fun getChildId(groupPosition: Int, childPosition: Int) = childPosition.toLong()
    override fun hasStableIds() = false

    override fun getGroupView(groupPosition: Int, isExpanded: Boolean, convertView: View?, parent: ViewGroup?): View {
        val tv = (convertView as? TextView) ?: TextView(ctx).apply {
            setPadding(dp(16), dp(14), dp(16), dp(14))
            textSize = 15f
            setTextColor(Color.parseColor("#3A2C1E"))
            setBackgroundColor(Color.parseColor("#F7F3EC"))
        }
        val g = groups[groupPosition]
        tv.text = if (g.items.isEmpty()) g.title
        else (if (isExpanded) "▾  " else "▸  ") + g.title
        return tv
    }

    override fun getChildView(groupPosition: Int, childPosition: Int, isLastChild: Boolean, convertView: View?, parent: ViewGroup?): View {
        val tv = (convertView as? TextView) ?: TextView(ctx).apply {
            setPadding(dp(32), dp(12), dp(16), dp(12))
            textSize = 15f
            setTextColor(Color.parseColor("#1F2937"))
            setBackgroundColor(Color.WHITE)
        }
        tv.text = groups[groupPosition].items[childPosition].title
        return tv
    }

    override fun isChildSelectable(groupPosition: Int, childPosition: Int) = true

    private fun dp(v: Int): Int = (v * ctx.resources.displayMetrics.density).toInt()
}
