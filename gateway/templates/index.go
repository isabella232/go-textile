package templates

const Index = `
<html>
    <head>
        <title>{{.root.String}}</title>
        <link href="/css/style.css" rel="stylesheet">
    </head>
    <body>
        <div class="title">Index of {{.root.String}}</div>
        <ul>
            <li><a href="{{.back}}">..</a></li>
            {{range .links}}
                <li><a href="{{.Path.String}}">/{{.Link.Name}}<span class="right">{{.Size}}</span></a></li>
            {{end}}
        </ul>
	</body>
</html>
`
