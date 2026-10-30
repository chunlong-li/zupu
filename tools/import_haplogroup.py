#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
从 PhyloTree.org 的 mtDNA 树 (Build 17) 生成 dna_haplogroup_mt.go。

用法：
  1. 下载 https://www.phylotree.org/builds/mtDNA_tree_Build_17.zip 解压得到
     "mtDNA tree Build 17.htm"，放到本目录（tools/）。
  2. 运行 python3 import_haplogroup.py，生成
     backend/internal/handler/dna_haplogroup_mt.go。

数据源：PhyloTree.org mtDNA tree Build 17 (2016)，约 1.28 万个单倍群节点。
更新方式：PhyloTree 发布新 Build 时，替换 htm 文件重跑本脚本即可。
"""
import re
import sys
from html.parser import HTMLParser


class _NamesParser(HTMLParser):
    """提取表格 td 中纯文本形式的单倍群名（排除 font 标签内的变异位点）。"""

    def __init__(self):
        super().__init__()
        self.in_td = 0
        self.in_font = 0
        self.cur = []
        self.names = set()

    def handle_starttag(self, tag, attrs):
        if tag == 'td':
            self.in_td += 1
            self.cur = []
        elif tag == 'font' and self.in_td:
            self.in_font += 1

    def handle_data(self, data):
        if self.in_td and not self.in_font:
            self.cur.append(data)

    def handle_endtag(self, tag):
        if tag == 'font' and self.in_font:
            self.in_font -= 1
        elif tag == 'td' and self.in_td:
            self.in_td -= 1
            t = ''.join(self.cur).strip()
            if re.fullmatch(r'[A-Za-z][A-Za-z0-9]{0,19}', t) and not re.match(r'^[ACGT][0-9]', t):
                self.names.add(t)


def canon(s):
    """与后端 canonicalHaplo 的 mtDNA 规范化一致：首字母大写、其余小写。"""
    if not s:
        return s
    return s[0].upper() + s[1:].lower()


# 字母跳变根节点的父（PhyloTree 命名不延续前缀，需按分子人类学共识手动指定）
MANUAL_PARENT = {
    'L0': 'L', 'L1': 'L', 'L2': 'L', 'L3': 'L', 'L4': 'L', 'L5': 'L', 'L6': 'L',
    'M': 'L3', 'N': 'L3',
    'D': 'M', 'E': 'M', 'G': 'M', 'Q': 'M',
    'C': 'M8', 'Z': 'M8',
    'A': 'N', 'I': 'N', 'O': 'N', 'S': 'N', 'W': 'N', 'X': 'N', 'Y': 'N9',
    'R': 'N',
    'B': 'R', 'F': 'R', 'P': 'R', 'U': 'R',
    'H': 'HV', 'V': 'HV', 'J': 'JT', 'T': 'JT', 'K': 'U8',
    'B2': 'B', 'B4': 'B', 'B5': 'B', 'B6': 'B',
    'HV': 'R0', 'JT': 'R0', 'U8': 'U', 'R0': 'R', 'U': 'R',
}


def load_names(htm_path):
    p = _NamesParser()
    with open(htm_path, encoding='windows-1252') as f:
        p.feed(f.read())
    return {canon(n) for n in p.names if n not in ('rCRS', 'reserved')}


def build_parent(names):
    manual = {canon(k): canon(v) for k, v in MANUAL_PARENT.items()}
    parent = {}
    for s in names:
        found = None
        for k in range(len(s) - 1, 0, -1):
            if s[:k] in names:
                found = s[:k]
                break
        if found:
            parent[s] = found
        elif s in manual:
            parent[s] = manual[s]
        else:
            print('警告：未处理孤儿节点 %r' % s, file=sys.stderr)
    # 补上 MANUAL_PARENT 里的虚拟节点（不在 names 里，如 B/HV/JT/R0/U8）
    for k, v in manual.items():
        if k not in parent:
            parent[k] = v
    parent['L'] = ''
    return parent


def generate_go(parent, out_path):
    lines = ['package handler', '',
             '// haploParentMT PhyloTree.org mtDNA 树 Build 17 完整父子关系（自动导入，约 %d 节点）。' % len(parent),
             '// 数据源：https://www.phylotree.org/ （PhyloTree mtDNA tree Build 17, 2016）',
             '// 由 tools/import_haplogroup.py 生成，如需更新替换 htm 后重跑本脚本即可。',
             'var haploParentMT = map[string]string{']
    for s in sorted(parent):
        lines.append('\t"%s": "%s",' % (s, parent[s]))
    lines.append('}')
    lines.append('')
    with open(out_path, 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines))
    print('生成 %s：%d 节点，%.1f KB' % (out_path, len(parent), len('\n'.join(lines)) / 1024))


def main():
    htm = 'mtDNA tree Build 17.htm'
    out = '../backend/internal/handler/dna_haplogroup_mt.go'
    names = load_names(htm)
    parent = build_parent(names)
    generate_go(parent, out)


if __name__ == '__main__':
    main()
