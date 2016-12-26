(function() {

  function bookmark(data) {
    var req = new window.XMLHttpRequest()
    req.open('POST', "/bookmarks", true)
    req.setRequestHeader('Content-Type', 'application/json')
    req.send(JSON.stringify(data))
  }

  var url = location.href
  var canonical = document.querySelector("link[rel='canonical']")
  if (canonical && canonical.href) {
    url = canonical.href
  }
  bookmark({title: document.title, url: url})

}())
