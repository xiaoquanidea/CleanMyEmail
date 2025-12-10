package folder

import (
	"strings"

	"CleanMyEmail/internal/model"
)

// BuildFolderTree 构建文件夹树
// 保持IMAP返回的顺序，同时构建父子关系
func BuildFolderTree(folders []*model.MailFolder) []*model.FolderTreeNode {
	if len(folders) == 0 {
		return nil
	}

	// 检测分隔符
	delimiter := detectDelimiter(folders)
	if delimiter == "" {
		// 没有分隔符，直接返回扁平列表
		return buildFlatTree(folders)
	}

	// 构建树形结构
	return buildHierarchicalTree(folders, delimiter)
}

// detectDelimiter 检测分隔符
func detectDelimiter(folders []*model.MailFolder) string {
	for _, f := range folders {
		if f.Delimiter != "" {
			return f.Delimiter
		}
	}
	return ""
}

// buildFlatTree 构建扁平树
func buildFlatTree(folders []*model.MailFolder) []*model.FolderTreeNode {
	nodes := make([]*model.FolderTreeNode, 0, len(folders))
	for _, f := range folders {
		nodes = append(nodes, &model.FolderTreeNode{
			Key:          f.FullPath,
			Label:        f.Name,
			FullPath:     f.FullPath,
			MessageCount: f.MessageCount,
			IsLeaf:       true,
			Disabled:     !f.IsSelectable,
		})
	}
	return nodes
}

// buildHierarchicalTree 构建层级树
func buildHierarchicalTree(folders []*model.MailFolder, delimiter string) []*model.FolderTreeNode {
	// 用于存储所有节点的映射
	nodeMap := make(map[string]*model.FolderTreeNode)
	// 根节点列表
	var roots []*model.FolderTreeNode
	// 记录顺序
	orderMap := make(map[string]int)

	for i, f := range folders {
		orderMap[f.FullPath] = i
		
		parts := strings.Split(f.FullPath, delimiter)
		
		// 创建或获取当前节点
		node := &model.FolderTreeNode{
			Key:          f.FullPath,
			Label:        parts[len(parts)-1],
			FullPath:     f.FullPath,
			MessageCount: f.MessageCount,
			IsLeaf:       true,
			Disabled:     !f.IsSelectable,
		}
		nodeMap[f.FullPath] = node

		if len(parts) == 1 {
			// 顶级节点
			roots = append(roots, node)
		} else {
			// 子节点，找到或创建父节点
			parentPath := strings.Join(parts[:len(parts)-1], delimiter)
			parent, exists := nodeMap[parentPath]
			if !exists {
				// 创建虚拟父节点
				parent = &model.FolderTreeNode{
					Key:      parentPath,
					Label:    parts[len(parts)-2],
					FullPath: parentPath,
					IsLeaf:   false,
					Disabled: true,
				}
				nodeMap[parentPath] = parent
				// 检查父节点是否是顶级
				if len(parts) == 2 {
					roots = append(roots, parent)
				}
			}
			parent.IsLeaf = false
			parent.Children = append(parent.Children, node)
		}
	}

	return roots
}

// GetAllFolderPaths 获取所有文件夹路径（包括子文件夹）
func GetAllFolderPaths(nodes []*model.FolderTreeNode) []string {
	var paths []string
	var collect func(nodes []*model.FolderTreeNode)
	collect = func(nodes []*model.FolderTreeNode) {
		for _, node := range nodes {
			if !node.Disabled {
				paths = append(paths, node.FullPath)
			}
			if len(node.Children) > 0 {
				collect(node.Children)
			}
		}
	}
	collect(nodes)
	return paths
}

// FindNodeByPath 根据路径查找节点
func FindNodeByPath(nodes []*model.FolderTreeNode, path string) *model.FolderTreeNode {
	for _, node := range nodes {
		if node.FullPath == path {
			return node
		}
		if len(node.Children) > 0 {
			if found := FindNodeByPath(node.Children, path); found != nil {
				return found
			}
		}
	}
	return nil
}

// GetChildPaths 获取节点及其所有子节点的路径
func GetChildPaths(node *model.FolderTreeNode) []string {
	var paths []string
	if !node.Disabled {
		paths = append(paths, node.FullPath)
	}
	for _, child := range node.Children {
		paths = append(paths, GetChildPaths(child)...)
	}
	return paths
}

